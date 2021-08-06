package main

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"reflect"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/kerwinzlb/GridTradingServer/common"
	"github.com/kerwinzlb/GridTradingServer/db"
	"github.com/kerwinzlb/GridTradingServer/ding"
	"github.com/kerwinzlb/GridTradingServer/log"
	"github.com/kerwinzlb/GridTradingServer/okex-sdk-api"
	"github.com/kerwinzlb/GridTradingServer/params"
	pb "github.com/kerwinzlb/GridTradingServer/proto"
	"google.golang.org/grpc"
)

const (
	MGO_DB_NAME                = "grid"
	MGO_COLLECTION_CONFIG_NAME = "config"
	MGO_COLLECTION_TICKET_NAME = "ticket"
	MGO_COLLECTION_ORDER_NAME  = "orders"
)

type Server struct {
	conf       *okex.Config
	dbConf     atomic.Value
	instId     string
	tickSzN    int
	lotSzN     int
	minSzN     int
	gridNum    int
	gridSize   float64
	restClient *okex.Client
	mgo        *db.Mongo
	mysql      *db.Mysql
	status     uint64

	sideChan chan string
	stop     chan struct{} // Channel to wait for termination notifications
}

func New(instId string, conf *okex.Config) (*Server, error) {
	s := &Server{
		conf:       conf,
		instId:     instId,
		restClient: okex.NewClient(*conf),
		sideChan:   make(chan string, 100),
		stop:       make(chan struct{}),
	}

	atomic.StoreUint64(&s.status, 0)
	pubRes, err := s.restClient.GetPublicInstruments(okex.SPOT, "", s.instId)
	if err != nil {
		return nil, err
	}
	log.Warn("New", "TickSz", pubRes.Data[0].TickSz, "LotSz", pubRes.Data[0].LotSz, "MinSz", pubRes.Data[0].MinSz)
	s.tickSzN = common.FloatRoundLen(pubRes.Data[0].TickSz)
	s.lotSzN = common.FloatRoundLen(pubRes.Data[0].LotSz)
	s.minSzN = common.FloatRoundLen(pubRes.Data[0].MinSz)

	err = s.dbConnect()
	if err != nil {
		return nil, err
	}

	dbConf, err := s.GetDbConfig()
	if err != nil {
		return nil, err
	}
	s.dbConf.Store(dbConf)
	return s, nil
}

func (s *Server) getSz(pric float64, side string, dbConf DbConfig) (string, string) {
	sz := ""
	if dbConf.Mode == 0 {
		if side == "buy" {
			sz = strconv.FormatFloat(dbConf.BuyNum, 'f', s.lotSzN, 64)
		} else if side == "sell" {
			sz = strconv.FormatFloat(dbConf.SellNum, 'f', s.lotSzN, 64)
		}
	} else if dbConf.Mode == 1 {
		if side == "buy" {
			sz = strconv.FormatFloat(dbConf.BuyAmt/(pric), 'f', s.lotSzN, 64)
		} else if side == "sell" {
			sz = strconv.FormatFloat(dbConf.SellAmt/(pric), 'f', s.lotSzN, 64)
		}
	}
	px := strconv.FormatFloat(pric, 'f', s.tickSzN, 64)
	return px, sz
}

func (s *Server) initPostOrder(dbFlag bool) error {
	ticRes, err := s.restClient.GetMarketTicker(s.instId)
	if err != nil {
		log.Error("initPostOrder GetMarketTicker", "err", err)
		return err
	}
	ts, _ := strconv.ParseInt(ticRes.Data[0].Ts, 10, 64)
	if ts >= params.TerminationTimeStamp {
		log.Error("initPostOrder Reach the service termination date！")
		return errors.New("Reach the service termination date！")
	}
	if dbFlag {
		s.InsertTicker(ticRes.Data[0])
	}
	last, _ := strconv.ParseFloat(ticRes.Data[0].Last, 64)
	index := 0
	dbConf := s.dbConf.Load().(DbConfig)
	gridSize := 1 + s.gridSize
	strList := strings.Split(s.instId, "-")
	prefix := strList[0] + strList[1]
	for i := s.gridNum; i > 0; i-- {
		pric := last / math.Pow((gridSize), float64(i))
		px, sz := s.getSz(pric, "buy", dbConf)
		clOrdId := hex.EncodeToString([]byte(strconv.Itoa(index - i)))
		_, err := s.PostBuyTradeOrder(prefix+clOrdId, px, sz)
		if err != nil {
			log.Error("initPostOrder PostBuyTradeOrder", "err", err)
			return err
		}

		pric = last * math.Pow((gridSize), float64(i))
		px, sz = s.getSz(pric, "sell", dbConf)
		clOrdId = hex.EncodeToString([]byte(strconv.Itoa(index + i)))
		_, err = s.PostSellTradeOrder(prefix+clOrdId, px, sz)
		if err != nil {
			log.Error("initPostOrder PostSellTradeOrder", "err", err)
			return err
		}
	}
	return nil
}

func (s *Server) MonitorLoop() {
	conf := s.dbConf.Load().(DbConfig)
	sec := time.Duration(conf.Sec)
	maxDiffNum := conf.MaxDiffNum
	getDbConfTic := time.NewTicker(30 * time.Second)
	secTic := time.NewTicker(sec * time.Second)
	defer getDbConfTic.Stop()
	defer secTic.Stop()

	buyNum := 0
	sellNum := 0

	for {
		select {
		case <-getDbConfTic.C:
			dbConf, err := s.GetDbConfig()
			if err != nil {
				log.Error("Monitor GetDbConfig", "err", err)
				continue
			}
			status := atomic.LoadUint64(&s.status)
			if dbConf.Status == 0 {
				log.Warn("MonitorLoop -> s.Stop() is called!")
				s.Stop()
				buyNum = 0
				sellNum = 0
			} else if dbConf.Status == 1 {
				if status == 1 {
					go s.monitorOrder()
					oldDbConf := s.dbConf.Load().(DbConfig)
					if !reflect.DeepEqual(oldDbConf, dbConf) {
						log.Warn("MonitorLoop -> dbConf is updated!", "oldDbConf", oldDbConf, "dbConf", dbConf)
						if sec != time.Duration(dbConf.Sec) {
							sec = time.Duration(dbConf.Sec)
							secTic = time.NewTicker(sec * time.Second)
							buyNum = 0
							sellNum = 0
						}
						maxDiffNum = dbConf.MaxDiffNum
						s.dbConf.Store(dbConf)
					}
				} else if status == 0 {
					log.Warn("MonitorLoop -> s.Start() is called!")
					s.dbConf.Store(dbConf)
					err := s.Start()
					if err != nil {
						log.Error("MonitorLoop s.Start", "err", err)
					}
					buyNum = 0
					sellNum = 0
				}
			}
		case <-secTic.C:
			buyNum = 0
			sellNum = 0
		case side := <-s.sideChan:
			if side == "buy" {
				buyNum++
			} else if side == "sell" {
				sellNum++
			}
			diffNum := buyNum - sellNum

			if int(math.Abs(float64(diffNum))) == maxDiffNum {
				log.Warn("MonitorLoop -> Risk control is triggered!", "buyNum", buyNum, "sellNum", sellNum, "maxDiffNum", maxDiffNum)
				buyNum = 0
				sellNum = 0
				s.shutdown()
			}
		case <-s.stop:
			return
		}
	}
}

func (s *Server) monitorOrder() {
	orderLen := 0
	n := 2
	for i := 0; i < n; i++ {
		trdp, err := s.GetTradeOrdersPending()
		if err != nil {
			log.Error("monitorOrder GetTradeOrdersPending", "err", err)
			return
		}
		orderLen = len(trdp.Data)
		if orderLen == s.gridNum*2 {
			return
		}
		log.Warn("monitorOrder", "orderSum", orderLen, "gridSum", s.gridNum*2)
		if i != n-1 {
			time.Sleep(600 * time.Millisecond)
		}
	}
	s.CancelAllOrders()
	s.initPostOrder(false)
	msg := fmt.Sprintf(s.instId+"交易对未成交订单数量:%d != 格子数量:%d, 撤掉所有未成交订单并重新挂单", orderLen, s.gridNum*2)
	ding.PostRobotMessage(s.conf.DingUrl, msg)
	log.Warn("monitorOrder triggered successfully!")
}

func (s *Server) grpcConnect() (pb.Ws_GetOrderInfoClient, error) {
	conn, err := grpc.Dial(s.conf.WsServerAddr+":"+strconv.Itoa(s.conf.WsServerPort), grpc.WithInsecure())
	// defer conn.Close()
	if err != nil {
		log.Error("grpcConnect grpc.Dial", "err", err)
		return nil, err
	}
	c := pb.NewWsClient(conn)

	req := new(pb.GetOrderRequest)
	req.InstId = s.instId

	return c.GetOrderInfo(context.Background(), req)
}

func (s *Server) WsRecvLoop() {
	stream, err := s.grpcConnect()
	if err != nil {
		log.Error("WsRecvLoop return grpcConnect", "err", err)
		return
	}
	for {
		select {
		case <-s.stop:
			log.Warn("WsRecvLoop return")
			return
		default:
		}

		r, err := stream.Recv()
		if err != nil {
			if err != io.EOF {
				log.Error("WsRecvLoop stream.Recv", "err", err)
				stream, err = s.grpcConnect()
				if err != nil {
					log.Error("WsRecvLoop grpcConnect", "err", err)
				}
			}
		} else {
			go s.ReceivedOrdersDataCallback(r.Replybody)
		}
	}
}

func (s *Server) Start() error {
	dbConf := s.dbConf.Load().(DbConfig)
	s.gridNum = dbConf.GridNum
	s.gridSize = dbConf.GridSize
	if dbConf.Status == 1 {
		err := s.initPostOrder(true)
		if err != nil {
			return err
		}
		atomic.StoreUint64(&s.status, 1)
	}
	return nil
}

func (s *Server) shutdown() {
	s.Stop()
	err := s.UpdateDbStatus()
	if err != nil {
		log.Error("shutdown UpdateOne", "err", err)
	}
	ding.PostRobotMessage(s.conf.DingUrl, s.instId+"交易对服务停止！")
}

func (s *Server) ReceivedOrdersDataCallback(rspMsg []byte) error {
	res := new(okex.WSOrdersResponse)
	err := okex.JsonBytes2Struct(rspMsg, res)
	if err != nil {
		log.Error("ReceivedOrdersDataCallback JsonBytes2Struct", "rspMsg", string(rspMsg), "err", err)
		return err
	}
	orders := make([]okex.DataOrder, 0)
	strList := strings.Split(s.instId, "-")
	prefix := strList[0] + strList[1]
	for _, order := range res.Data {
		ts, _ := strconv.ParseInt(order.CTime, 10, 64)
		if ts >= params.TerminationTimeStamp {
			log.Warn("ReceivedOrdersDataCallback Reach the service termination date！")
			s.Exit()
			return errors.New("Reach the service termination date！")
		}
		if order.InstType == okex.SPOT && order.OrdType == "post_only" {
			if order.State == "filled" {
				dbConf := s.dbConf.Load().(DbConfig)
				gridSize := 1 + s.gridSize
				clOrdId, _ := hex.DecodeString(strings.TrimLeft(order.ClOrdId, prefix))
				index, _ := strconv.Atoi(string(clOrdId))
				orders = append(orders, order)
				pri, _ := strconv.ParseFloat(order.Px, 64)
				s.sideChan <- order.Side
				if order.Side == "buy" {
					pric := pri * gridSize
					px, sz := s.getSz(pric, "sell", dbConf)
					clOrdId := hex.EncodeToString([]byte(strconv.Itoa(index + 1)))
					_, err := s.PostSellTradeOrder(prefix+clOrdId, px, sz)
					if err != nil {
						log.Error("买单成交，挂卖单失败", "error", err)
						return err
					}

					pric = pri / math.Pow(gridSize, float64(s.gridNum))
					px, sz = s.getSz(pric, "buy", dbConf)
					clOrdId = hex.EncodeToString([]byte(strconv.Itoa(index - s.gridNum)))
					_, err = s.PostBuyTradeOrder(prefix+clOrdId, px, sz)
					if err != nil {
						log.Error("买单成交，挂买单失败", "error", err)
						return err
					}
					clOrdId = hex.EncodeToString([]byte(strconv.Itoa(index + s.gridNum + 1)))
					_, err = s.restClient.PostTradeCancelOrder(s.instId, "", prefix+clOrdId)
					if err != nil {
						log.Error("买单成交，撤销卖单失败", "error", err)
						return err
					}
				} else if order.Side == "sell" {
					pric := pri / gridSize
					px, sz := s.getSz(pric, "buy", dbConf)
					clOrdId := hex.EncodeToString([]byte(strconv.Itoa(index - 1)))
					_, err := s.PostBuyTradeOrder(prefix+clOrdId, px, sz)
					if err != nil {
						log.Error("卖单成交，挂买单失败", "error", err)
						return err
					}
					pric = pri * math.Pow(gridSize, float64(s.gridNum))
					px, sz = s.getSz(pric, "sell", dbConf)
					clOrdId = hex.EncodeToString([]byte(strconv.Itoa(index + s.gridNum)))
					_, err = s.PostSellTradeOrder(prefix+clOrdId, px, sz)
					if err != nil {
						log.Error("卖单成交，挂卖单失败", "error", err)
						return err
					}
					clOrdId = hex.EncodeToString([]byte(strconv.Itoa(index - s.gridNum - 1)))
					_, err = s.restClient.PostTradeCancelOrder(s.instId, "", prefix+clOrdId)
					if err != nil {
						log.Error("卖单成交，撤销买单失败", "error", err)
						return err
					}
				}
			}
		}
	}

	s.InsertOrders(orders)

	return nil
}

func (s *Server) PostTradeCancelBatchOrders(ordIds, clOrdIds []string) error {
	reqs := make([]map[string]string, 0)
	for i := 0; i < len(ordIds); i++ {
		req := okex.NewParams()
		req["instId"] = s.instId
		req["ordId"] = ordIds[i]
		req["clOrdId"] = clOrdIds[i]
		reqs = append(reqs, req)
	}
	_, err := s.restClient.PostTradeCancelBatchOrders(&reqs)
	if err != nil {
		return err
	}
	return nil
}

func (s *Server) PostBuyTradeOrder(clOrdId, px, sz string) (string, error) {
	req := okex.NewParams()
	req["instId"] = s.instId
	req["tdMode"] = "cash"
	// req["ccy"] = "USDT"
	req["clOrdId"] = clOrdId
	req["side"] = "buy"
	req["ordType"] = "post_only"
	req["px"] = strings.TrimRight(strings.TrimRight(px, "0"), ".")
	req["sz"] = sz
	res, err := s.restClient.PostTradeOrder(&req)
	if err != nil {
		log.Error("PostBuyTradeOrder", "Code", res.Code, "Msg", res.Msg, "SCode", res.Data[0].SCode, "sMsg", res.Data[0].SMsg)
		return "", err
	}
	return res.Data[0].OrdId, nil
}

func (s *Server) PostSellTradeOrder(clOrdId, px, sz string) (string, error) {
	req := okex.NewParams()
	req["instId"] = s.instId
	req["tdMode"] = "cash"
	// req["ccy"] = "USDT"
	req["clOrdId"] = clOrdId
	req["side"] = "sell"
	req["ordType"] = "post_only"
	req["px"] = strings.TrimRight(strings.TrimRight(px, "0"), ".")
	req["sz"] = sz
	res, err := s.restClient.PostTradeOrder(&req)
	if err != nil {
		log.Error("PostSellTradeOrder", "Code", res.Code, "Msg", res.Msg, "SCode", res.Data[0].SCode, "sMsg", res.Data[0].SMsg)
		return "", err
	}
	return res.Data[0].OrdId, nil
}

func (s *Server) GetTradeOrdersPending() (*okex.PendingOrderResult, error) {
	req := okex.NewReqParams()
	req.AddParam("instType", okex.SPOT)
	req.AddParam("instId", s.instId)
	req.AddParam("ordType", "post_only")
	return s.restClient.GetTradeOrdersPending(req)
}

func (s *Server) CancelAllOrders() {
	trdp, err := s.GetTradeOrdersPending()
	if err != nil {
		log.Error("CancelAllOrders GetTradeOrdersPending", "err", err)
		return
	}
	for _, order := range trdp.Data {
		_, err := s.restClient.PostTradeCancelOrder(s.instId, order.OrdId, "")
		if err != nil {
			log.Error("CancelAllOrders PostTradeCancelOrder", "err", err)
		}
	}
}

func (s *Server) Stop() error {
	s.CancelAllOrders()
	atomic.StoreUint64(&s.status, 0)
	return nil
}

func (s *Server) Exit() error {
	s.Stop()
	s.dbDisConnect()
	close(s.stop)
	return nil
}

// Wait blocks the thread until the node is stopped. If the node is not running
// at the time of invocation, the method immediately returns.
func (s *Server) Wait() {
	stop := s.stop

	<-stop
	os.Exit(1)
}
