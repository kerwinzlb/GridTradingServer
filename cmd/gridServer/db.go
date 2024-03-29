package main

import (
	"errors"
	"fmt"
	"strconv"

	"github.com/kerwinzlb/GridTradingServer/db"
	"github.com/kerwinzlb/GridTradingServer/log"
	"github.com/kerwinzlb/GridTradingServer/okex-sdk-api"
	"go.mongodb.org/mongo-driver/bson"
)

var ErrDbType = errors.New("DbType is invalid")

const (
	mongo int = iota
	mysql
)

func (s *Server) dbConnect() error {
	if s.conf.DbType == mongo {
		s.mgo = db.NewMgo(s.conf.DbEndpoint)
		return s.mgo.Connect()
	} else if s.conf.DbType == mysql {
		s.mysql = db.NewMysql(s.conf.DbEndpoint)
		return s.mysql.Connect()
	} else {
		return ErrDbType
	}
}

func (s *Server) dbDisConnect() error {
	if s.conf.DbType == mongo {
		return s.mgo.DisConnect()
	} else if s.conf.DbType == mysql {
		return s.mysql.DisConnect()
	} else {
		return ErrDbType
	}
}

func (s *Server) GetDbConfig() (DbConfig, error) {
	var dbConf DbConfig
	if s.conf.DbType == mongo {
		err := s.mgo.FindOne(MGO_DB_NAME, MGO_COLLECTION_CONFIG_NAME, bson.M{"instId": s.instId}, &dbConf)
		return dbConf, err
	} else if s.conf.DbType == mysql {
		cmdSql := fmt.Sprintf("select * from "+MGO_COLLECTION_CONFIG_NAME+" where instId = '%s'", s.instId)
		err := s.mysql.QueryRow(cmdSql, &dbConf.InstId, &dbConf.BuyAmt, &dbConf.SellAmt, &dbConf.BuyNum, &dbConf.SellNum, &dbConf.GridSize, &dbConf.GridNum, &dbConf.Mode, &dbConf.Sec, &dbConf.MaxDiffNum, &dbConf.Status)
		return dbConf, err
	} else {
		return DbConfig{}, ErrDbType
	}
}

func (s *Server) InsertTicker(ticket okex.Ticket) error {
	last, _ := strconv.ParseFloat(ticket.Last, 64)
	ts, _ := strconv.ParseInt(ticket.Ts, 10, 64)
	if s.conf.DbType == mongo {
		err := s.mgo.InsertOne(MGO_DB_NAME, MGO_COLLECTION_TICKET_NAME, bson.M{"instId": ticket.InstId, "last": last, "ts": ts})
		if err != nil {
			log.Error("InsertTicker mgo.InsertOne", "err", err)
		}
		return err
	} else if s.conf.DbType == mysql {
		cmdSql := fmt.Sprintf("INSERT INTO "+MGO_COLLECTION_TICKET_NAME+" (instId, last, ts) VALUES ('%s', '%f', '%d')", ticket.InstId, last, ts)
		_, err := s.mysql.Execute(cmdSql)
		if err != nil {
			log.Error("InsertTicker mysql.Execute", "cmdSql", cmdSql, "err", err)
		}
		return err
	} else {
		return ErrDbType
	}
}

func (s *Server) InsertOrders(orders []okex.DataOrder) {
	for _, order := range orders {
		px, _ := strconv.ParseFloat(order.Px, 64)
		sz, _ := strconv.ParseFloat(order.Sz, 64)
		accFillSz, _ := strconv.ParseFloat(order.AccFillSz, 64)
		avgPx, _ := strconv.ParseFloat(order.AvgPx, 64)
		fee, _ := strconv.ParseFloat(order.Fee, 64)
		fillTime, _ := strconv.ParseInt(order.FillTime, 10, 64)
		cTime, _ := strconv.ParseInt(order.CTime, 10, 64)
		if s.conf.DbType == mongo {
			err := s.mgo.InsertOne(MGO_DB_NAME, MGO_COLLECTION_ORDER_NAME, bson.M{"instId": order.InstId, "ordId": order.OrdId, "clOrdId": order.ClOrdId, "side": order.Side, "px": px, "sz": sz, "accFillSz": accFillSz, "avgPx": avgPx, "fee": fee, "fillTime": fillTime, "cTime": cTime})
			if err != nil {
				log.Error("InsertOrders mgo.InsertOne", "err", err)
			}
		} else if s.conf.DbType == mysql {
			cmdSql := fmt.Sprintf("INSERT INTO "+MGO_COLLECTION_ORDER_NAME+" (instId, ordId, clOrdId, side, px, sz, accFillSz, avgPx, fee, fillTime, cTime) VALUES ('%s', '%s', '%s', '%s', '%f', '%f', '%f', '%f', '%f', '%d', '%d')", order.InstId, order.OrdId, order.ClOrdId, order.Side, px, sz, accFillSz, avgPx, fee, fillTime, cTime)
			_, err := s.mysql.Execute(cmdSql)
			if err != nil {
				log.Error("InsertOrders mysql.Execute", "cmdSql", cmdSql, "err", err)
			}
		} else {
			log.Error("InsertOrders", "err", ErrDbType)
		}
	}
	return
}

func (s *Server) UpdateDbStatus() error {
	if s.conf.DbType == mongo {
		_, err := s.mgo.UpdateOne(MGO_DB_NAME, MGO_COLLECTION_CONFIG_NAME, bson.M{"instId": s.instId}, bson.M{"status": 0})
		if err != nil {
			log.Error("UpdateDbStatus mgo.UpdateOne", "err", err)
		}
		return err
	} else if s.conf.DbType == mysql {
		// update userinfo set username = 'anson' where uid = 1
		cmdSql := fmt.Sprintf("update "+MGO_COLLECTION_CONFIG_NAME+" set status = '0' where instId = '%s'", s.instId)
		_, err := s.mysql.Execute(cmdSql)
		if err != nil {
			log.Error("UpdateDbStatus mysql.Execute", "cmdSql", cmdSql, "err", err)
		}
		return err
	} else {
		return ErrDbType
	}
}
