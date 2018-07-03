package services

import (
	"sync"
	"utils"
	"dao"
	"rpcs"
	"entities"
	"time"
)

/***
	充值服务：启动子协程扫描链，过滤充值交易，并提交到notify服务等待其稳定
	子协程（startScanChain）：扫描区块，找到充值交易
 */
type depositService struct {
	BaseService
	sync.Once
	addresses []string
	height uint64
}

var _depositService *depositService

func GetDepositService() *depositService {
	if _depositService == nil {
		_depositService = new(depositService)
		_depositService.Once = sync.Once {}
		_depositService.Once.Do(func() {
			_depositService.create()
		})
	}
	return _depositService
}

func (service *depositService) create() error {
	service.name = "depositService"
	service.status.RegAsObs(service)
	return service.BaseService.create()
}

func (service *depositService) BeforeTurn(s *utils.Status, tgtStt int) {
	var err error
	switch tgtStt {
	case INIT:
		utils.LogMsgEx(utils.INFO, "initialization", nil)
		// 加载所有内部地址
		if err = service.loadAddresses(); err != nil {
			panic(utils.LogMsgEx(utils.ERROR, "加载地址失败：%v", err))
		}
		// 获取上一次扫描的块高
		if err = service.getCurrentHeight(); err != nil {
			panic(utils.LogMsgEx(utils.ERROR, "获取当前块高失败：%v", err))
		}
	case START:
		utils.LogMsgEx(utils.INFO, "start", nil)
	}
}

func (service *depositService) AfterTurn(s *utils.Status, srcStt int) {
	switch s.Current() {
	case INIT:
		utils.LogMsgEx(utils.INFO, "initialized", nil)
	case START:
		// 开启协程扫描区块链上的交易记录
		go service.startScanChain()
		utils.LogMsgEx(utils.INFO, "started", nil)
	}
}

func (service *depositService) loadAddresses() error {
	coinSetting := utils.GetConfig().GetCoinSettings()
	var err error
	service.addresses, err = dao.GetAddressDAO().FindInuseByAsset(coinSetting.Name)
	return err
}

func (service *depositService) getCurrentHeight() error {
	coinSetting := utils.GetConfig().GetCoinSettings()
	var err error
	var height int64
	if height, err = dao.GetHeightDAO().GetHeight(coinSetting.Name); err != nil {
		utils.LogMsgEx(utils.ERROR, "查询不到块高：%v", err)
		return nil
	}
	if height == 0 {
		if _, err = dao.GetHeightDAO().ChkOrAddAsset(coinSetting.Name); err != nil {
			return err
		}
	}
	service.height = uint64(height)
	return nil
}

func (service *depositService) startScanChain() {
	var err error
	coinSet := utils.GetConfig().GetCoinSettings()
	rpc := rpcs.GetRPC(coinSet.Name)
	for err == nil && service.status.Current() == START {
		utils.LogMsgEx(utils.INFO, "块高: %d", service.height)

		// 获取指定高度的交易
		var txs []entities.Transaction
		if txs, err = rpc.GetTransactions(uint(service.height)); err != nil {
			utils.LogMsgEx(utils.ERROR, "获取交易失败：%v", err)
			continue
		}

		for _, tx := range txs {
			// 如果充值地址不属于钱包，跳过
			if !utils.StrArrayContains(service.addresses, tx.To) {
				continue
			}

			utils.LogMsgEx(utils.INFO, "发现交易：%v", tx)
			deposit := entities.TurnTxToDeposit(&tx)

			// 持久化到数据库
			if _, err = dao.GetDepositDAO().AddScannedDeposit(&deposit); err != nil {
				utils.LogMsgEx(utils.ERROR, "添加未稳定提币记录失败：%v", err)
				continue
			}

			// 获取当前块高
			var curHeight uint64
			if curHeight, err = rpc.GetCurrentHeight(); err != nil {
				utils.LogMsgEx(utils.ERROR, "获取块高失败：%v", err)
				continue
			}
			dao.GetProcessDAO().SaveProcess(&entities.DatabaseProcess {
				entities.BaseProcess {
					deposit.TxHash,
					deposit.Asset,
					entities.DEPOSIT,
					entities.NOTIFY,
					true,
				},
				deposit.Height,
				curHeight,
				deposit.Height + uint64(coinSet.Stable),
				time.Now(),
			})

			// 如果已经达到稳定块高，直接存入数据库
			if deposit.Height + uint64(coinSet.Stable) >= curHeight {
				utils.LogMsgEx(utils.INFO, "交易（%s）进入稳定状态", deposit.TxHash)

				if err = TxIntoStable(tx.TxHash, curHeight); err != nil {
					utils.LogMsgEx(utils.ERROR, "插入稳定交易记录失败：%v", err)
					continue
				}
			} else {
				// 未进入稳定状态，抛给通知等待服务
				toNotifySig <- tx
				utils.LogMsgEx(utils.INFO, "交易（%s）进入等待列队", deposit.TxHash)
			}
		}

		// 持久化高度到height表
		if service.height % 20 == 0 {
			if _, err = dao.GetHeightDAO().UpdateHeight(coinSet.Name, service.height); err != nil {
				utils.LogMsgEx(utils.ERROR, "更新块高失败：%v", err)
				continue
			}
			utils.LogMsgEx(utils.INFO, "块高更新到：%d", service.height)
		}
		service.height++
	}
	close(toNotifySig)
	service.status.TurnTo(DESTORY)
}