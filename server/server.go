package server

import (
	"context"
	"encoding/hex"
	"math"
	"os"
	"strconv"
	"strings"
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
	restEndpoint string
	restClient   *okex.Client
	wsEndpoint   string
	wsClient     *okex.OKWSAgent
	mgoEndpoint  string
	mgoClient    *mongo.Client
	mgoCancel    context.CancelFunc
	ctx          context.Context

	stop chan struct{} // Channel to wait for termination notifications
}

func New(instId string, conf *okex.Config) (*Server, error) {
	confCopy := *conf
	s := &Server{
		conf:         &confCopy,
		instId:       instId,
		gridSize:     1 + conf.GridSize,
		gridNum:      conf.GridNum,
		restEndpoint: conf.Endpoint,
		restClient:   okex.NewClient(*conf),
		wsEndpoint:   conf.WSEndpoint,
		wsClient:     new(okex.OKWSAgent),
		mgoEndpoint:  conf.MgoEndpoint,
		mgoClient:    new(mongo.Client),
		stop:         make(chan struct{}),
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

func (s *Server) insertOrders(index int, instid, side, px, sz, avgPx, fee, fillTime, cTime string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	collection := s.mgoClient.Database(s.conf.MgoDBName).Collection(s.conf.MgoCollectionName[0])
	collection.InsertOne(ctx, bson.M{"index": index, "instid": instid, "side": side, "px": px, "sz": sz, "avgPx": avgPx, "fee": fee, "fillTime": fillTime, "cTime": cTime})
	return nil
}

func (s *Server) Start() error {
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

	last, _ := strconv.ParseFloat(s.last, 64)
	index := 0
	for i := s.gridNum; i > 0; i-- {
		px := strconv.FormatFloat(last/math.Pow(s.gridSize, float64(i)), 'f', s.tickSzN, 64)
		clOrdId := hex.EncodeToString([]byte(strconv.Itoa(index - i)))
		_, px, err := s.PostBuyTradeOrder(clOrdId, px, s.sz)
		if err != nil {
			return err
		}

		px = strconv.FormatFloat(last*math.Pow(s.gridSize, float64(i)), 'f', s.tickSzN, 64)
		clOrdId = hex.EncodeToString([]byte(strconv.Itoa(index + i)))
		_, px, err = s.PostSellTradeOrder(clOrdId, px, s.sz)
		if err != nil {
			return err
		}
	}
	return nil
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
			clOrdId, _ := hex.DecodeString(res.Data[0].ClOrdId)
			index, _ := strconv.Atoi(string(clOrdId))
			s.insertOrders(index, res.Data[0].InstId, res.Data[0].Side, res.Data[0].Px, res.Data[0].Sz, res.Data[0].AvgPx, res.Data[0].Fee, res.Data[0].FillTime, res.Data[0].CTime)
			sellPri, _ := strconv.ParseFloat(res.Data[0].Px, 64)
			buyPri, _ := strconv.ParseFloat(res.Data[0].Px, 64)
			if res.Data[0].Side == "buy" {
				log.Debug("买单成交")
				sellPri = sellPri * s.gridSize
				px := strconv.FormatFloat(sellPri, 'f', s.tickSzN, 64)
				clOrdId := hex.EncodeToString([]byte(strconv.Itoa(index + 1)))
				_, px, err := s.PostSellTradeOrder(clOrdId, px, s.sz)
				if err != nil {
					log.Error("买单成交，挂卖单失败", "error", err)
					return err
				}
				log.Debug("买单成交，挂卖单成功")

				for i := 0; i < s.gridNum; i++ {
					buyPri = buyPri / s.gridSize
					sellPri = sellPri * s.gridSize
				}
				px = strconv.FormatFloat(buyPri, 'f', s.tickSzN, 64)
				clOrdId = hex.EncodeToString([]byte(strconv.Itoa(index - s.gridNum)))
				_, px, err = s.PostBuyTradeOrder(clOrdId, px, s.sz)
				if err != nil {
					log.Error("买单成交，挂买单失败", "error", err)
					return err
				}
				log.Debug("买单成交，挂买单成功")
				px = strings.TrimRight(strings.TrimRight(strconv.FormatFloat(sellPri, 'f', s.tickSzN, 64), "0"), ".")
				clOrdId = hex.EncodeToString([]byte(strconv.Itoa(index + s.gridNum + 1)))
				_, err = s.restClient.PostTradeCancelOrder(s.instId, "", clOrdId)
				if err != nil {
					log.Error("买单成交，撤销卖单失败", "error", err)
					return err
				}
				log.Debug("买单成交，撤销卖单成功")
			} else if res.Data[0].Side == "sell" {
				log.Debug("卖单成交")
				buyPri = buyPri / s.gridSize
				px := strconv.FormatFloat(buyPri, 'f', s.tickSzN, 64)
				clOrdId := hex.EncodeToString([]byte(strconv.Itoa(index - 1)))
				_, px, err := s.PostBuyTradeOrder(clOrdId, px, s.sz)
				if err != nil {
					log.Error("卖单成交，挂买单失败", "error", err)
					return err
				}
				log.Debug("卖单成交，挂买单成功")
				for i := 0; i < s.gridNum; i++ {
					buyPri = buyPri / s.gridSize
					sellPri = sellPri * s.gridSize
				}
				px = strconv.FormatFloat(sellPri, 'f', s.tickSzN, 64)
				clOrdId = hex.EncodeToString([]byte(strconv.Itoa(index + s.gridNum)))
				_, px, err = s.PostSellTradeOrder(clOrdId, px, s.sz)
				if err != nil {
					log.Error("卖单成交，挂卖单失败", "error", err)
					return err
				}
				log.Debug("卖单成交，挂卖单成功")
				px = strings.TrimRight(strings.TrimRight(strconv.FormatFloat(buyPri, 'f', s.tickSzN, 64), "0"), ".")
				clOrdId = hex.EncodeToString([]byte(strconv.Itoa(index - s.gridNum - 1)))
				_, err = s.restClient.PostTradeCancelOrder(s.instId, "", clOrdId)
				if err != nil {
					log.Error("卖单成交，撤销买单失败", "error", err)
					return err
				}
				log.Debug("卖单成交，撤销买单成功")
			}
		}
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

func (s *Server) PostBuyTradeOrder(clOrdId, px, sz string) (string, string, error) {
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

func (s *Server) PostSellTradeOrder(clOrdId, px, sz string) (string, string, error) {
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
	s.UnSubscribe()
	s.wsClient.Stop()

	trdp, err := s.restClient.GetTradeOrdersPending(okex.NewReqParams())
	if err != nil {
		log.Error("Server stop", "GetTradeOrdersPending err", err)
	}
	for key, val := range *trdp {
		if key == "data" {
			orders := val.([]map[string]string)
			for _, order := range orders {
				s.restClient.PostTradeCancelOrder(s.instId, order["ordId"], "")
			}
			break
		}
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
