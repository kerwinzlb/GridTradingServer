#!/bin/bash　　

HOSTNAME="localhost"                                           #数据库信息
PORT="3306"
USERNAME="root"

DBNAME="grid"                                                       #数据库名称
TABLENAME="config"                                            #数据库中表的名称

select_sql="select instId from ${TABLENAME}"
depts=`mysql -h${HOSTNAME}  -P${PORT}  -u${USERNAME} ${DBNAME} -e "${select_sql}"|tail -n +2`
mPath=$HOME/gridTradingServer
for d in $depts; do
	mPath=$HOME/gridTradingServer/$d
	if [ ! -d $mPath ];then
		mkdir $mPath
	fi
	cd $mPath
	pids=`ps aux | grep $d | grep -v grep | awk '{print $2}'`
	if [ ${pids} ]
	then
		echo $d '在运行中'
	else
		cp ../gridServer .
		cp ../config.json .
		logN=$d"_""log"
		time1=$(date "+%Y%m%d%H%M%S")
		mv $logN $logN"_"$time1
		nohup ./gridServer --instid $d > $logN &
	fi
done
