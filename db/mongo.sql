db.order.aggregate([{$group:{"_id":{"instid":"$instid","side":"$side"},"count":{$sum:1}, "amtSum":{$sum:{$multiply:["$px","$sz"]}}}}])

db.config.insert({"instId":"ETH-USDT","buyAmt":10,"sellAmt":10,"buyNum":0,"sellNum":0,"buyGridSize":0.005,"sellGridSize":0.005,"gridNum":1,"mode":1,"upperLimit":10000000,"lowerLimit":0,"status":1})

db.config.update({instId:"DOT-USDT"}, {$set:{'status':0}})

db.config.update({instId:"DOT-USDT"}, {$set:{'buyAmt':10, 'sellAmt':10, 'buyGridSize':0.005, 'sellGridSize':0.005}})