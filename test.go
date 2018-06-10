package main

import (
	"fmt"
	"net/http"
	"encoding/json"
	"io/ioutil"
	"bytes"
	"log"
	"sync"
	"time"
	"database/sql"
	_ "github.com/go-sql-driver/mysql"
	"utils"
)

const URL = "http://18.144.17.127:8545"
type ReqBody struct {
	Method string	`json:method`
	Params []string	`json:params`
	Id string		`json:id`
}

var sig = make(chan int)
var end = make(chan bool)
func testChannel() {
	for i := range sig {
		time.Sleep(2000 * time.Millisecond)
		log.Println(i)
	}
	end <- true
	log.Println("End")
}
func testChannelByOK() {
	for {
		i, ok := <- sig
		if !ok { break }
		time.Sleep(2000 * time.Millisecond)
		log.Println(i)
	}
	log.Println("End")
}

const DBUrl			= ""
const DBName		= "test"
const DBUserName	= "root"
const DBPassword	= "59524148chenOP"

const DropTable		= "DROP TABLE IF EXISTS user"
const CreateTable	= `CREATE TABLE IF NOT EXISTS user (
	id INTEGER NOT NULL AUTO_INCREMENT,
	username VARCHAR(20) NOT NULL UNIQUE,
	password VARCHAR(20) NOT NULL,
	PRIMARY KEY(username),
	INDEX(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8`
const InsertTest	= "INSERT INTO user (username, password) VALUES (?, ?)"
const DeleteTest	= "DELETE FROM user WHERE username=?"
const UpdateTest	= "UPDATE user SET password=? WHERE username=?"
const SelectTest	= "SELECT username, password FROM user"
const SelOneTest	= SelectTest + " WHERE username=?"

func displayAll(db *sql.DB) bool {
	rows, err := db.Query(SelectTest)
	if err != nil {
		log.Fatal(err)
		return false
	}
	for rows.Next() {
		var username string
		var password string
		if err := rows.Scan(&username, &password); err != nil {
			log.Fatal(err)
			continue
		}
		log.Printf("username: %s, password: %s\n", username, password)
	}
	if err := rows.Err(); err != nil {
		log.Fatal(err)
		return false
	}
	return true
}

type TestObs struct {
}

func (o *TestObs) BeforeTurn(s *utils.Status) {
	log.Printf("Before turn: %d\n", s.Current())
}

func (o *TestObs) AfterTurn(s *utils.Status) {
	log.Printf("After turn: %d\n", s.Current())
}

func main() {
	fmt.Println("abcd")

	// Request from blockchain
	reqBody := ReqBody { "eth_blockNumber", []string {}, "latest" }
	reqStr, err := json.Marshal(reqBody)
	if err != nil {
		log.Fatal(err)
		return
	}
	reqBuf := bytes.NewBuffer([]byte(reqStr))
	res, err := http.Post(URL, "application/json", reqBuf)
	defer res.Body.Close()

	// Parse response body
	bodyStr, err := ioutil.ReadAll(res.Body)
	log.Println(string(bodyStr))

	// Test goroutine
	wg := new(sync.WaitGroup)
	wg.Add(2)
	for i := 0; i < 2; i++ {
		go func(id int) {
			log.Println(id);
			defer wg.Done()
		}(i)
	}
	wg.Wait()

	// Test channel
	fmt.Println()
	go testChannel()
	for i := 0; i < 5; i++ {
		sig <- i
	}
	close(sig)
	<- end
	close(end)

	//Use of-idiom test channel
	fmt.Println()
	sig = make(chan int)
	go testChannelByOK()
	sig <- 20
	sig <- 30
	sig <- 40
	close(sig)
	time.Sleep(5 * time.Second)

	//Connect database
	var db *sql.DB
	var insertTest *sql.Stmt
	var tx *sql.Tx
	var result sql.Result
	var affectedRows [20]int64

	fmt.Println()
	if db, err = sql.Open("mysql", fmt.Sprintf("%s:%s@%s/%s?charset=utf8&tls=skip-verify",
		DBUserName, DBPassword, DBUrl, DBName)); err != nil {
		log.Fatal(err)
		return
	}
	if err := db.Ping(); err != nil {
		log.Fatal(err)
		return
	}

	//Drop and create table
	if _, err = db.Exec(DropTable); err != nil {
		log.Fatal(err)
		return
	}
	if _, err = db.Exec(CreateTable); err != nil {
		log.Fatal(err)
		return
	}
	log.Println("Create table succeed")

	//Test insert and transaction
	if insertTest, err = db.Prepare(InsertTest); err != nil {
		log.Fatal(err)
		return
	}
	// updateTest, err := db.Prepare(UpdateTest)
	// if err != nil {
	// 	log.Fatal(err)
	// 	return
	// }

	if tx, err = db.Begin(); err != nil {
		log.Fatal(err)
		return
	}
	
	if result, err = tx.Stmt(insertTest).Exec("opteacher", "12345"); err != nil {
		log.Fatal(err)
		tx.Rollback()
		return
	}
	affectedRows[0], _ = result.RowsAffected()
	if result, err = tx.Stmt(insertTest).Exec("tyoukasin", "54321"); err != nil {
		log.Fatal(err)
		tx.Rollback()
		return
	}
	affectedRows[1], _ = result.RowsAffected()
	if err = tx.Commit(); err != nil {
		log.Fatal(err)
		tx.Rollback()
		return
	}
	log.Printf("Insert table succeed, affected %d rows\n", affectedRows[0] + affectedRows[1])
	if !displayAll(db) { return }

	//Test update
	if result, err = db.Exec(UpdateTest, "abcde", "opteacher"); err != nil {
		log.Fatal(err)
		return
	}
	lastUpdateId, _ := result.RowsAffected()
	log.Printf("Update table succeed, affected %d rows\n", lastUpdateId)
	if !displayAll(db) { return }

	//Test delete
	if result, err = db.Exec(DeleteTest, "tyoukasin"); err != nil {
		log.Fatal(err)
		return
	}
	lastDeleteId, _ := result.RowsAffected()
	log.Printf("Delete table record succeed, affected %d rows\n", lastDeleteId)
	if !displayAll(db) { return }

	//Test status and observer
	fmt.Println()
	o := TestObs {}
	const (
		NONE = iota
		INIT
		START
	)
	a := utils.Status {
		AllStatus:	[]int { NONE, INIT, START },
		StatusVal:	NONE,
		Observers:	[]utils.Observer { &o },
	}
	log.Println(a.Current())

	a.TurnTo(START)
	log.Println(a.Current())
}