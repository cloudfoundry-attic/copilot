#!/bin/bash

set -eu
set -o pipefail

cd $(dirname $0)/..

function bootPostgres {
	echo -n "booting postgres"
	(/docker-entrypoint.sh postgres &> /var/log/postgres-boot.log) &
	trycount=0
	for i in `seq 1 30`; do
		set +e
		psql -h localhost -U postgres -c '\conninfo' &>/dev/null
		exitcode=$?
		set -e
		if [ $exitcode -eq 0 ]; then
			echo "connection established to postgres"
			return 0
		fi
		echo -n "."
		sleep 1
	done
	echo "unable to connect to postgres"
	exit 1
}

function bootMysql {
	echo -n "booting mysql"
	(MYSQL_ROOT_PASSWORD=password  /entrypoint.sh mysqld &> /var/log/mysql-boot.log) &
	trycount=0
	for i in `seq 1 60`; do
		set +e
		echo '\s;' | mysql -h 127.0.0.1 -u root --password="password" &>/dev/null
		exitcode=$?
		set -e
		if [ $exitcode -eq 0 ]; then
			echo "connection established to mysql"
			return 0
		fi
		echo -n "."
		sleep 1
	done
	echo "unable to connect to mysql"
	exit 1
}

echo "starting getting ginkgo"
go get github.com/onsi/ginkgo
go install github.com/onsi/ginkgo/ginkgo

if [ "${1:-""}" = "" ]; then
  extraArgs=""
else
  extraArgs="${@}"
fi

echo "starting getting dependencies"
go get -u github.com/golang/dep/cmd/dep
dep ensure

if [ ${DB:-"none"} = "mysql" ] || [ ${DB:-"none"} = "mysql-5.6" ]; then
  bootMysql
  ginkgo -r --race -randomizeAllSpecs ${extraArgs} db/timeouts
elif [ ${DB:-"none"} = "postgres" ]; then
  bootPostgres
  ginkgo -r --race -randomizeAllSpecs ${extraArgs} db/timeouts
else
  echo "skipping database"
  extraArgs="-skipPackage=db ${extraArgs}"
fi

ginkgo -r -p --race -randomizeAllSpecs -randomizeSuites -skipPackage=timeouts ${extraArgs}

rm -rf vendor/
