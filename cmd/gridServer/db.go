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
		s.msql = db.NewMysql(s.conf.DbEndpoint)
		return s.msql.Connect()
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
		res := make([]interface{}, 0)
		res = append(res, &dbConf.InstId)
		res = append(res, &dbConf.BuyAmt)
		res = append(res, &dbConf.SellAmt)
		res = append(res, &dbConf.BuyNum)
		res = append(res, &dbConf.SellNum)
		res = append(res, &dbConf.BuyGridSize)
		res = append(res, &dbConf.SellGridSize)
		res = append(res, &dbConf.GridNum)
		res = append(res, &dbConf.Mode)
		res = append(res, &dbConf.Sec)
		res = append(res, &dbConf.MaxDiffNum)
		res = append(res, &dbConf.Status)
		cmdSql := "select * from " + MGO_COLLECTION_CONFIG_NAME + " where instId = " + s.instId
		err := s.msql.QueryRow(cmdSql, res)
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
		_, err := s.msql.Execute(cmdSql)
		if err != nil {
			log.Error("InsertTicker msql.Execute", "cmdSql", cmdSql, "err", err)
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
		avgPx, _ := strconv.ParseFloat(order.AvgPx, 64)
		fee, _ := strconv.ParseFloat(order.Fee, 64)
		fillTime, _ := strconv.ParseInt(order.FillTime, 10, 64)
		cTime, _ := strconv.ParseInt(order.CTime, 10, 64)
		if s.conf.DbType == mongo {
			err := s.mgo.InsertOne(MGO_DB_NAME, MGO_COLLECTION_ORDER_NAME, bson.M{"instId": order.InstId, "ordId": order.OrdId, "clOrdId": order.ClOrdId, "side": order.Side, "px": px, "sz": sz, "avgPx": avgPx, "fee": fee, "fillTime": fillTime, "cTime": cTime})
			if err != nil {
				log.Error("InsertOrders mgo.InsertOne", "err", err)
			}
		} else if s.conf.DbType == mysql {
			cmdSql := fmt.Sprintf("INSERT INTO "+MGO_COLLECTION_ORDER_NAME+" (instId, ordId, clOrdId, side, px, sz, avgPx, fee, fillTime, cTime) VALUES ('%s', '%s', '%s', '%s', '%f', '%f', '%f', '%f', '%d', '%d')", order.InstId, order.OrdId, order.ClOrdId, order.Side, px, sz, avgPx, fee, fillTime, cTime)
			_, err := s.msql.Execute(cmdSql)
			if err != nil {
				log.Error("InsertOrders msql.Execute", "cmdSql", cmdSql, "err", err)
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
		_, err := s.msql.Execute(cmdSql)
		if err != nil {
			log.Error("UpdateDbStatus msql.Execute", "cmdSql", cmdSql, "err", err)
		}
		return err
	} else {
		return ErrDbType
	}
}
