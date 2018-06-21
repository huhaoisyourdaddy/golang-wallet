package dao

import (
	"sync"
	"databases"
	"database/sql"
	"utils"
	"errors"
)

type addressDao struct {
	baseDao
	sync.Once
}

var _addressDao *addressDao

func GetAddressDAO() *addressDao {
	if _addressDao == nil {
		_addressDao = new(addressDao)
		_addressDao.Once = sync.Once {}
		_addressDao.Once.Do(func() {
			_addressDao.create("address")
		})
	}
	return _addressDao
}

func (dao *addressDao) newAddress(asset string, address string, inuse bool) (int64, error) {
	var db *sql.DB
	var err error
	if db, err = databases.ConnectMySQL(); err != nil {
		panic(utils.LogIdxEx(utils.ERROR, 0010, err))
	}

	var result sql.Result
	var insertSQL string
	var ok bool
	var useSQL = "NewAddress"
	if inuse {
		useSQL = "NewAddressInuse"
	}
	if insertSQL, ok = dao.sqls[useSQL]; ok {
		if result, err = db.Exec(insertSQL, asset, address); err != nil {
			panic(utils.LogIdxEx(utils.ERROR, 0012, err))
		}
	} else {
		return 0, utils.LogIdxEx(utils.ERROR, 0011, errors.New(useSQL))
	}
	return result.RowsAffected()
}

func (dao *addressDao) NewAddress(asset string, address string) (int64, error) {
	return dao.newAddress(asset, address, false)
}

func (dao *addressDao) NewAddressInuse(asset string, address string) (int64, error) {
	return dao.newAddress(asset, address, true)
}

func (dao *addressDao) FindInuseByAsset(asset string) ([]string, error) {
	var db *sql.DB
	var err error
	if db, err = databases.ConnectMySQL(); err != nil {
		panic(utils.LogIdxEx(utils.ERROR, 0010, err))
	}

	var selectSQL string
	var ok bool
	if selectSQL, ok = dao.sqls["FindByAsset"]; !ok {
		return []string {}, utils.LogIdxEx(utils.ERROR, 0011, errors.New("FindByAsset"))
	}

	var rows *sql.Rows
	if rows, err = db.Query(selectSQL, asset); err != nil {
		panic(utils.LogIdxEx(utils.ERROR, 0013, err))
	}
	defer rows.Close()

	var addresses []string
	for rows.Next() {
		var address string
		if err = rows.Scan(&address); err != nil {
			utils.LogIdxEx(utils.ERROR, 0014, err)
			continue
		}
		addresses = append(addresses, address)
	}
	if err = rows.Err(); err != nil {
		panic(utils.LogIdxEx(utils.ERROR, 0014, err))
	}
	return addresses, nil
}