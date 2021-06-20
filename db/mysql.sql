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


INSERT INTO config (instId, buyAmt,sellAmt,buyNum,sellNum,gridSize,gridNum,mode,sec,maxDiffNum,status) VALUES ("AAVE-USDT", 10, 10, 0, 0, 0.007, 1, 1, 30000, 10000, 1)