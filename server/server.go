package server

import (
	"context"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/kerwinzlb/GridTradingServer/common"
	"github.com/kerwinzlb/GridTradingServer/log"
	"github.com/kerwinzlb/GridTradingServer/okex-sdk-api"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type Server struct {
	conf         *okex.Config
	instId       string
	last         string
	sz           string
	tickSzN      int
	lotSzN       int
	minSzN       int
	gridSize     float64
	gridNum      int
	gridOrdidMap map[int64]string
	gridIndexMap map[string]int64
	restEndpoint string
	restClient   *okex.Client
	wsEndpoint   string
	wsClient     *okex.OKWSAgent
	mgoEndpoint  string
	mgoClient    *mongo.Client
	mgoCancel    context.CancelFunc
	ctx          context.Context

	stop chan struct{} // Channel to wait for termination notifications
	lock *sync.RWMutex
}

func New(instId string, conf *okex.Config) (*Server, error) {
	confCopy := *conf
	s := &Server{
		conf:         &confCopy,
		instId:       instId,
		gridSize:     1 + conf.GridSize,
		gridNum:      conf.GridNum,
		gridOrdidMap: make(map[int64]string),
		gridIndexMap: make(map[string]int64),
		restEndpoint: conf.Endpoint,
		restClient:   okex.NewClient(*conf),
		wsEndpoint:   conf.WSEndpoint,
		wsClient:     new(okex.OKWSAgent),
		mgoEndpoint:  conf.MgoEndpoint,
		mgoClient:    new(mongo.Client),
		stop:         make(chan struct{}),
		lock:         new(sync.RWMutex),
	}
	err := s.newMgo()
	if err != nil {
		return nil, err
	}
	ticRes, err := s.restClient.GetMarketTicker(s.instId)
	if err != nil {
		return nil, err
	}
	s.last = ticRes.Data[0].Last
	s.insertTicker(ticRes.Data[0].InstId, ticRes.Data[0].Last, ticRes.Data[0].Ts)
	pubRes, err := s.restClient.GetPublicInstruments("SPOT", "", s.instId)
	if err != nil {
		return nil, err
	}
	s.tickSzN = common.FloatRoundLen(pubRes.Data[0].TickSz)
	s.lotSzN = common.FloatRoundLen(pubRes.Data[0].LotSz)
	s.minSzN = common.FloatRoundLen(pubRes.Data[0].MinSz)
	pric, _ := strconv.ParseFloat(ticRes.Data[0].Last, 64)
	s.sz = strconv.FormatFloat(s.conf.Amount/pric, 'f', s.lotSzN, 64)
	return s, nil
}

func (s *Server) newMgo() error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	client, err := mongo.Connect(ctx, options.Client().ApplyURI(s.mgoEndpoint))
	if err != nil {
		return err
	}
	s.mgoClient = client
	s.mgoCancel = cancel
	s.ctx = ctx
	return nil
}

func (s *Server) insertTicker(instid, last, ts string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	collection := s.mgoClient.Database(s.conf.MgoDBName).Collection(s.conf.MgoCollectionName[1])
	collection.InsertOne(ctx, bson.M{"instid": instid, "last": last, "ts": ts})
	return nil
}

func (s *Server) insertOrders(index int64, instid, side, px, sz, avgPx, fee, fillTime, cTime string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	collection := s.mgoClient.Database(s.conf.MgoDBName).Collection(s.conf.MgoCollectionName[0])
	collection.InsertOne(ctx, bson.M{"index": index, "instid": instid, "side": side, "px": px, "sz": sz, "avgPx": avgPx, "fee": fee, "fillTime": fillTime, "cTime": cTime})
	return nil
}

func (s *Server) Start() error {
	buyPri, _ := strconv.ParseFloat(s.last, 64)
	sellPri, _ := strconv.ParseFloat(s.last, 64)
	index := int64(0)
	for i := int64(0); i < int64(s.gridNum); i++ {
		buyPri = buyPri / s.gridSize
		px := strconv.FormatFloat(buyPri, 'f', s.tickSzN, 64)
		ordId, px, err := s.PostBuyTradeOrder(px, s.sz)
		if err != nil {
			return err
		}
		s.setGridMap(index-(i+1), ordId)

		sellPri = sellPri * s.gridSize
		px = strconv.FormatFloat(sellPri, 'f', s.tickSzN, 64)
		ordId, px, err = s.PostSellTradeOrder(px, s.sz)
		if err != nil {
			return err
		}
		s.setGridMap(index+(i+1), ordId)
	}
	s.wsClient.Start(s.conf)
	err := s.wsClient.Login(s.conf.ApiKey, s.conf.Passphrase)
	if err != nil {
		return err
	}
	log.Info("websocket Login success")
	err = s.Subscribe()
	if err != nil {
		return err
	}
	log.Info("websocket Subscribe success")
	return nil
}

func (s *Server) setGridMap(index int64, ordId string) {
	s.lock.Lock()
	defer s.lock.Unlock()

	s.gridOrdidMap[index] = ordId
	s.gridIndexMap[ordId] = index
	return
}

func (s *Server) getGridOrdidMap(index int64) string {
	s.lock.RLock()
	defer s.lock.RUnlock()

	return s.gridOrdidMap[index]
}

func (s *Server) getGridindexMap(ordid string) int64 {
	s.lock.RLock()
	defer s.lock.RUnlock()

	return s.gridIndexMap[ordid]
}

func (s *Server) deleteGridMap(index int64, ordId string) {
	s.lock.Lock()
	defer s.lock.Unlock()

	delete(s.gridOrdidMap, index)
	delete(s.gridIndexMap, ordId)
	return
}

func (s *Server) Subscribe() error {
	err := s.wsClient.Subscribe(okex.CHNL_OEDERS, "SPOT", s.ReceivedOrdersDataCallback)
	if err != nil {
		return err
	}
	return nil
}

func (s *Server) UnSubscribe() error {
	err := s.wsClient.UnSubscribe(okex.CHNL_OEDERS, "SPOT")
	if err != nil {
		return err
	}
	return nil
}

func (s *Server) ReceivedOrdersDataCallback(obj interface{}) error {
	res := obj.(*okex.WSOrdersResponse)
	if len(res.Data) == 0 {
		return nil
	}
	if res.Data[0].Code == "0" && res.Data[0].InstType == "SPOT" && res.Data[0].InstId == s.instId && res.Data[0].OrdType == "post_only" {
		if res.Data[0].State == "filled" {
			index := s.getGridindexMap(res.Data[0].OrdId)
			s.deleteGridMap(index, res.Data[0].OrdId)
			s.insertOrders(index, res.Data[0].InstId, res.Data[0].Side, res.Data[0].Px, res.Data[0].Sz, res.Data[0].AvgPx, res.Data[0].Fee, res.Data[0].FillTime, res.Data[0].CTime)
			sellPri, _ := strconv.ParseFloat(res.Data[0].Px, 64)
			buyPri, _ := strconv.ParseFloat(res.Data[0].Px, 64)
			if res.Data[0].Side == "buy" {
				log.Debug("买单成交")
				//增加一个卖单
				sellPri = sellPri * s.gridSize
				px := strconv.FormatFloat(sellPri, 'f', s.tickSzN, 64)
				ordId, px, err := s.PostSellTradeOrder(px, s.sz)
				if err != nil {
					log.Error("买单成交，挂卖单失败", "ordId", ordId, "error", err)
					return err
				}
				s.setGridMap(index+1, ordId)
				log.Debug("买单成交，挂卖单成功")

				for i := 0; i < s.gridNum; i++ {
					buyPri = buyPri / s.gridSize
					sellPri = sellPri * s.gridSize
				}
				//增加一个买单
				px = strconv.FormatFloat(buyPri, 'f', s.tickSzN, 64)
				ordId, px, err = s.PostBuyTradeOrder(px, s.sz)
				if err != nil {
					log.Error("买单成交，挂买单失败", "ordId", ordId, "error", err)
					return err
				}
				s.setGridMap(index-int64(s.gridNum), ordId)
				log.Debug("买单成交，挂买单成功")
				//撤销一个卖单
				px = strings.TrimRight(strings.TrimRight(strconv.FormatFloat(sellPri, 'f', s.tickSzN, 64), "0"), ".")
				OrdId := s.getGridOrdidMap(index + int64(s.gridNum) + 1)
				err = s.PostTradeCancelOrder(OrdId, "")
				if err != nil {
					log.Error("买单成交，撤销卖单失败", "ordId", ordId, "error", err)
					return err
				}
				s.deleteGridMap(index+int64(s.gridNum)+1, OrdId)
				log.Debug("买单成交，撤销卖单成功")
			} else if res.Data[0].Side == "sell" {
				log.Debug("卖单成交")
				//增加一个买单
				buyPri = buyPri / s.gridSize
				px := strconv.FormatFloat(buyPri, 'f', s.tickSzN, 64)
				ordId, px, err := s.PostBuyTradeOrder(px, s.sz)
				if err != nil {
					log.Error("卖单成交，挂买单失败", "ordId", ordId, "error", err)
					return err
				}
				s.setGridMap(index-1, ordId)
				log.Debug("卖单成交，挂买单成功")
				for i := 0; i < s.gridNum; i++ {
					buyPri = buyPri / s.gridSize
					sellPri = sellPri * s.gridSize
				}
				//增加一个卖单
				px = strconv.FormatFloat(sellPri, 'f', s.tickSzN, 64)
				ordId, px, err = s.PostSellTradeOrder(px, s.sz)
				if err != nil {
					log.Error("卖单成交，挂卖单失败", "ordId", ordId, "error", err)
					return err
				}
				s.setGridMap(index+int64(s.gridNum), ordId)
				log.Debug("卖单成交，挂卖单成功")
				//撤销一个买单
				px = strings.TrimRight(strings.TrimRight(strconv.FormatFloat(buyPri, 'f', s.tickSzN, 64), "0"), ".")
				OrdId := s.getGridOrdidMap(index - int64(s.gridNum) - 1)
				err = s.PostTradeCancelOrder(OrdId, "")
				if err != nil {
					log.Error("卖单成交，撤销买单失败", "ordId", ordId, "error", err)
					return err
				}
				s.deleteGridMap(index-int64(s.gridNum)-1, OrdId)
				log.Debug("卖单成交，撤销买单成功")
			}
		}
	}
	return nil
}

func (s *Server) PostTradeCancelOrder(ordId, clOrdId string) error {
	_, err := s.restClient.PostTradeCancelOrder(s.instId, ordId, clOrdId)
	if err != nil {
		return err
	}
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

func (s *Server) PostBuyTradeOrder(px, sz string) (string, string, error) {
	// log.Debug("PostBuyTradeOrder px sz", px, sz)
	req := okex.NewParams()
	req["instId"] = s.instId
	req["tdMode"] = "cash"
	req["side"] = "buy"
	req["ordType"] = "post_only"
	req["px"] = strings.TrimRight(strings.TrimRight(px, "0"), ".")
	req["sz"] = sz
	res, err := s.restClient.PostTradeOrder(&req)
	if err != nil {
		return "", "", err
	}
	return res.Data[0].OrdId, px, nil
}

func (s *Server) PostSellTradeOrder(px, sz string) (string, string, error) {
	// log.Debug("PostSellTradeOrder px sz", px, sz)
	req := okex.NewParams()
	req["instId"] = s.instId
	req["tdMode"] = "cash"
	req["side"] = "sell"
	req["ordType"] = "post_only"
	req["px"] = strings.TrimRight(strings.TrimRight(px, "0"), ".")
	req["sz"] = sz
	res, err := s.restClient.PostTradeOrder(&req)
	if err != nil {
		return "", "", err
	}
	return res.Data[0].OrdId, px, nil
}

func (s *Server) Stop() error {
	s.lock.Lock()
	defer s.lock.Unlock()

	s.UnSubscribe()
	s.wsClient.Stop()
	for _, val := range s.gridOrdidMap {
		s.PostTradeCancelOrder(val, "")
	}

	s.mgoClient.Disconnect(s.ctx)
	s.mgoCancel()
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
