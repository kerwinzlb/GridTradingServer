db.order.aggregate([{$group:{"_id":{"instId":"$instId","side":"$side"},"count":{$sum:1}, "amtSum":{$sum:{$multiply:["$px","$sz"]}}}}])

db.config.insert({"instId":"ETH-USDT","buyAmt":10,"sellAmt":10,"buyNum":0,"sellNum":0,"gridSize":0.007,"gridNum":1,"mode":1,"sec":300,"maxDiffNum":15,"status":1})

db.config.update({instId:"DOT-USDT"}, {$set:{'status':0}})

db.config.update({instId:"DOT-USDT"}, {$set:{'buyAmt':10, 'sellAmt':10, 'gridSize':0.005}})