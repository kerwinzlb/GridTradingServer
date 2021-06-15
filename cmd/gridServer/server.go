package main

import (
	"context"
	"encoding/hex"
	"io"
	"math"
	"os"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/kerwinzlb/GridTradingServer/common"
	"github.com/kerwinzlb/GridTradingServer/db"
	"github.com/kerwinzlb/GridTradingServer/log"
	"github.com/kerwinzlb/GridTradingServer/okex-sdk-api"
	pb "github.com/kerwinzlb/GridTradingServer/proto"
	"google.golang.org/grpc"
)

const (
	MGO_DB_NAME                = "grid"
	MGO_COLLECTION_CONFIG_NAME = "config"
	MGO_COLLECTION_TICKET_NAME = "ticket"
	MGO_COLLECTION_ORDER_NAME  = "order"
)

type Server struct {
	conf       *okex.Config
	dbConf     atomic.Value
	instId     string
	tickSzN    int
	lotSzN     int
	minSzN     int
	gridNum    int
	restClient *okex.Client
	mgo        *db.Mongo
	msql       *db.Mysql
	status     uint64
	lock       *sync.RWMutex

	sideChan chan string
	stop     chan struct{} // Channel to wait for termination notifications
}

func New(instId string, conf *okex.Config) (*Server, error) {
	s := &Server{
		conf:       conf,
		instId:     instId,
		restClient: okex.NewClient(*conf),
		lock:       new(sync.RWMutex),
		sideChan:   make(chan string, 100),
		stop:       make(chan struct{}),
	}

	atomic.StoreUint64(&s.status, 0)
	pubRes, err := s.restClient.GetPublicInstruments(okex.SPOT, "", s.instId)
	if err != nil {
		return nil, err
	}
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

func (s *Server) initPostOrder() error {
	ticRes, err := s.restClient.GetMarketTicker(s.instId)
	if err != nil {
		return err
	}
	s.InsertTicker(ticRes.Data[0])
	last, _ := strconv.ParseFloat(ticRes.Data[0].Last, 64)
	index := 0
	dbConf := s.dbConf.Load().(DbConfig)
	for i := s.gridNum; i > 0; i-- {
		pric := last / math.Pow((1+dbConf.BuyGridSize), float64(i))
		px, sz := s.getSz(pric, "buy", dbConf)
		clOrdId := hex.EncodeToString([]byte(strconv.Itoa(index - i)))

		_, err := s.PostBuyTradeOrder(strings.Split(s.instId, "-")[0]+clOrdId, px, sz)
		if err != nil {
			return err
		}

		pric = last * math.Pow((1+dbConf.SellGridSize), float64(i))
		px, sz = s.getSz(pric, "sell", dbConf)
		px = strconv.FormatFloat(pric, 'f', s.tickSzN, 64)
		clOrdId = hex.EncodeToString([]byte(strconv.Itoa(index + i)))
		_, err = s.PostSellTradeOrder(strings.Split(s.instId, "-")[0]+clOrdId, px, sz)
		if err != nil {
			return err
		}
	}
	return nil
}

func (s *Server) MonitorLoop() {
	sec := time.Duration(300)
	getDbConfTic := time.NewTicker(15 * time.Second)
	moniOrdNumTic := time.NewTicker(30 * time.Second)
	secTic := time.NewTicker(sec * time.Second)
	defer getDbConfTic.Stop()
	defer moniOrdNumTic.Stop()
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
			if status == 0 {
				if dbConf.Status == 1 {
					err := s.Start()
					if err != nil {
						log.Error("MonitorLoop s.Start", "err", err)
					}
					buyNum = 0
					sellNum = 0
					secTic = time.NewTicker(sec * time.Second)
				}
			} else if status == 1 {
				if dbConf.Status == 0 {
					s.Stop()
					secTic.Stop()
					buyNum = 0
					sellNum = 0
				} else if dbConf.Status == 1 {
					dbConf := s.dbConf.Load().(DbConfig)
					if !reflect.DeepEqual(dbConf, dbConf) {
						sec = time.Duration(dbConf.Sec)
						s.dbConf.Store(dbConf)
					}
				}
			}
		case <-moniOrdNumTic.C:
			s.monitorOrder()
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
			if diffNum == -10 || diffNum == 10 {
				buyNum = 0
				sellNum = 0
				s.shutdown()
				secTic.Stop()
				secTic = time.NewTicker(sec * time.Second)
			}
		case <-s.stop:
			return
		}
	}
}

func (s *Server) monitorOrder() {
	trdp, err := s.GetTradeOrdersPending()
	if err != nil {
		log.Error("monitorOrder GetTradeOrdersPending", "err", err)
	}
	if len(trdp.Data) != s.gridNum*2 {
		s.CancelAllOrders()
		s.initPostOrder()
		log.Error("monitorOrder triggered successfully!")
	}
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
			log.Info("WsRecvLoop return")
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
	err := s.initPostOrder()
	if err != nil {
		return err
	}
	atomic.StoreUint64(&s.status, 1)
	return nil
}

func (s *Server) shutdown() {
	s.Stop()
	err := s.UpdateDbStatus()
	if err != nil {
		log.Error("shutdown UpdateOne", "err", err)
	}
}

func (s *Server) ReceivedOrdersDataCallback(rspMsg []byte) error {
	res := new(okex.WSOrdersResponse)
	err := okex.JsonBytes2Struct(rspMsg, res)
	if err != nil {
		log.Error("ReceivedOrdersDataCallback JsonBytes2Struct", "rspMsg", string(rspMsg), "err", err)
		return err
	}
	orders := make([]okex.DataOrder, 0)
	for _, order := range res.Data {
		if order.Code == "0" && order.InstType == okex.SPOT && order.OrdType == "post_only" {
			if order.State == "filled" {
				dbConf := s.dbConf.Load().(DbConfig)
				buyGridSize := 1 + dbConf.BuyGridSize
				sellGridSize := 1 + dbConf.SellGridSize
				clOrdId, _ := hex.DecodeString(strings.Trim(order.ClOrdId, s.instId))
				index, _ := strconv.Atoi(string(clOrdId))
				orders = append(orders, order)
				pri, _ := strconv.ParseFloat(order.Px, 64)
				s.sideChan <- order.Side
				if order.Side == "buy" {
					log.Debug("买单成交")
					pric := pri * sellGridSize
					px, sz := s.getSz(pric, "sell", dbConf)
					clOrdId := hex.EncodeToString([]byte(strconv.Itoa(index + 1)))
					_, err := s.PostSellTradeOrder(strings.Split(s.instId, "-")[0]+clOrdId, px, sz)
					if err != nil {
						log.Error("买单成交，挂卖单失败", "error", err)
						return err
					}
					log.Debug("买单成交，挂卖单成功")

					pric = pri / math.Pow(buyGridSize, float64(s.gridNum))
					px, sz = s.getSz(pric, "buy", dbConf)
					clOrdId = hex.EncodeToString([]byte(strconv.Itoa(index - s.gridNum)))
					_, err = s.PostBuyTradeOrder(strings.Split(s.instId, "-")[0]+clOrdId, px, sz)
					if err != nil {
						log.Error("买单成交，挂买单失败", "error", err)
						return err
					}
					log.Debug("买单成交，挂买单成功")
					clOrdId = hex.EncodeToString([]byte(strconv.Itoa(index + s.gridNum + 1)))
					_, err = s.restClient.PostTradeCancelOrder(s.instId, "", strings.Split(s.instId, "-")[0]+clOrdId)
					if err != nil {
						log.Error("买单成交，撤销卖单失败", "error", err)
						return err
					}
					log.Debug("买单成交，撤销卖单成功")
				} else if order.Side == "sell" {
					log.Debug("卖单成交")
					pric := pri / buyGridSize
					px, sz := s.getSz(pric, "buy", dbConf)
					clOrdId := hex.EncodeToString([]byte(strconv.Itoa(index - 1)))
					_, err := s.PostBuyTradeOrder(strings.Split(s.instId, "-")[0]+clOrdId, px, sz)
					if err != nil {
						log.Error("卖单成交，挂买单失败", "error", err)
						return err
					}
					log.Debug("卖单成交，挂买单成功")
					pric = pri * math.Pow(sellGridSize, float64(s.gridNum))
					px, sz = s.getSz(pric, "sell", dbConf)
					clOrdId = hex.EncodeToString([]byte(strconv.Itoa(index + s.gridNum)))
					_, err = s.PostSellTradeOrder(strings.Split(s.instId, "-")[0]+clOrdId, px, sz)
					if err != nil {
						log.Error("卖单成交，挂卖单失败", "error", err)
						return err
					}
					log.Debug("卖单成交，挂卖单成功")
					clOrdId = hex.EncodeToString([]byte(strconv.Itoa(index - s.gridNum - 1)))
					_, err = s.restClient.PostTradeCancelOrder(s.instId, "", strings.Split(s.instId, "-")[0]+clOrdId)
					if err != nil {
						log.Error("卖单成交，撤销买单失败", "error", err)
						return err
					}
					log.Debug("卖单成交，撤销买单成功")
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
	s.mgo.DisConnect()
	s.Stop()
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
