db.order.aggregate([{$group:{"_id":{"instid":"$instid","side":"$side"},"count":{$sum:1}, "amtSum":{$sum:{$multiply:["$px","$sz"]}}}}])

db.config.insert({instId:"DOT-USDT","buyAmt":3,"sellAmt":3,"buyNum":0,"sellNum":0,"buyGridSize":0.003,"sellGridSize":0.003,"gridNum":5,"mode":1,"stopSec":1,"stopCnt":1,"status":1})

db.config.update({instId:"DOT-USDT"}, {$set:{'status':0}})