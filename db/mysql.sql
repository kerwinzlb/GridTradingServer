CREATE TABLE ticket(
instId VARCHAR(40),
last DOUBLE,
ts BIGINT
)ENGINE=InnoDB DEFAULT CHARSET=utf8;

CREATE TABLE orders(
instId VARCHAR(40),
ordId VARCHAR(70),
clOrdId VARCHAR(70),
side VARCHAR(10),
px DOUBLE,
sz DOUBLE,
avgPx DOUBLE,
fee DOUBLE,
fillTime BIGINT,
cTime BIGINT
)ENGINE=InnoDB DEFAULT CHARSET=utf8;

CREATE TABLE config(
instId VARCHAR(40),
buyAmt DOUBLE,
sellAmt DOUBLE,
buyNum DOUBLE,
sellNum DOUBLE,
gridSize DOUBLE,
gridNum INT,
mode INT,
sec BIGINT,
maxDiffNum INT,
status INT
)ENGINE=InnoDB DEFAULT CHARSET=utf8;


INSERT INTO config (instId, buyAmt,sellAmt,buyNum,sellNum,gridSize,gridNum,mode,sec,maxDiffNum,status) VALUES ("ETH-USDT", 10, 10, 0.003, 0.003, 0.005, 1, 1, 300, 10, 1)