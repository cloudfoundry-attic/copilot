package timeouts_test

import (
	"context"
	"fmt"
	"math/rand"
	"os/exec"
	"strconv"
	"time"

	"code.cloudfoundry.org/cf-networking-helpers/db"
	"code.cloudfoundry.org/cf-networking-helpers/testsupport"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
)

var createTable = `CREATE TABLE IF NOT EXISTS mytable ( id SERIAL PRIMARY KEY);`
var testTimeoutInSeconds = float64(5)

var _ = Describe("Timeout", func() {
	var (
		dbConf   db.Config
		ctx      context.Context
		database *db.ConnWrapper
	)
	dbConf = testsupport.GetDBConfig()

	BeforeEach(func() {
		dbConf.DatabaseName = fmt.Sprintf("test_%x", rand.Int())
	})

	beginTx := func() error {
		_, err := database.BeginTx(ctx, nil)
		return err
	}

	queryRowContext := func() error {
		var databaseName string
		return database.QueryRowContext(ctx, "SELECT database();").Scan(&databaseName)
	}

	queryContext := func() error {
		_, err := database.QueryContext(ctx, "SELECT id FROM mytable;")
		return err
	}

	execContext := func() error {
		_, err := database.ExecContext(ctx, "INSERT into mytable (id) values (1);")
		return err
	}

	begin := func() error {
		_, err := database.Begin()
		return err
	}

	queryRow := func() error {
		var databaseName string
		return database.QueryRow("SELECT database();").Scan(&databaseName)
	}

	query := func() error {
		_, err := database.Query("SELECT id FROM mytable;")
		return err
	}

	exec := func() error {
		_, err := database.Exec("INSERT into mytable (id) values (1);")
		return err
	}

	expectContextDeadlineExceeded := func(dbFunc func() error) {
		It("returns a context deadline exceeded error", func(done Done) {
			defer database.Close()
			err := dbFunc()
			Expect(err).To(HaveOccurred())
			Expect(err).To(BeAssignableToTypeOf(context.DeadlineExceeded))
			close(done)
		}, testTimeoutInSeconds)
	}

	expectInvalidConnection := func(dbFunc func() error) {
		It("returns a tcp i/o timeout error", func(done Done) {
			defer database.Close()
			err := dbFunc()
			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError("invalid connection"))
			close(done)
		}, testTimeoutInSeconds)
	}

	AfterEach(func() {
		testsupport.RemoveDatabase(dbConf)
	})

	blockPort := func(port uint16) {
		portString := strconv.Itoa(int(port))
		By("blocking access to port " + portString)
		mustSucceed("iptables", "-A", "INPUT", "-p", "tcp", "--dport", portString, "-j", "DROP")
	}

	unblockPort := func(port uint16) {
		portString := strconv.Itoa(int(port))
		By("unblocking access to port " + portString)
		mustSucceed("iptables", "-D", "INPUT", "-p", "tcp", "--dport", portString, "-j", "DROP")
	}

	Describe("mysql", func() {
		if dbConf.Type != "mysql" {
			fmt.Printf("skipping mysql tests for db: %s\n", dbConf.Type)
			return
		}

		Context("when the read timeout is greater than the context timeout and the database is unreachable", func() {
			BeforeEach(func() {
				ctx, _ = context.WithTimeout(context.Background(), 2*time.Second)
				dbConf.Timeout = 3
				testsupport.CreateDatabase(dbConf)

				var err error
				database, err = db.GetConnectionPool(dbConf, context.Background())
				Expect(err).NotTo(HaveOccurred())

				By("creating a table")
				_, err = database.Exec(createTable)
				Expect(err).NotTo(HaveOccurred())

				blockPort(dbConf.Port)
			})

			AfterEach(func() {
				unblockPort(dbConf.Port)
			})

			Describe("QueryRowContext", func() {
				expectContextDeadlineExceeded(queryRowContext)
			})

			Describe("QueryContext", func() {
				expectContextDeadlineExceeded(queryContext)
			})

			Describe("ExecContext", func() {
				expectContextDeadlineExceeded(execContext)
			})

			Describe("BeginTx", func() {
				expectContextDeadlineExceeded(beginTx)
			})
		})

		Context("when the connect and read timeouts are set and the database is unreachable", func() {
			BeforeEach(func() {
				dbConf.Timeout = 1
				testsupport.CreateDatabase(dbConf)

				var err error
				database, err = db.GetConnectionPool(dbConf, context.Background())
				Expect(err).NotTo(HaveOccurred())

				By("creating a table")
				_, err = database.Exec(createTable)
				Expect(err).NotTo(HaveOccurred())

				blockPort(dbConf.Port)
			})

			AfterEach(func() {
				unblockPort(dbConf.Port)
			})

			Context("when the context has no deadline", func() {
				BeforeEach(func() {
					ctx = context.Background()
				})
				Describe("QueryRowContext", func() {
					expectInvalidConnection(queryRowContext)
				})

				Describe("QueryContext", func() {
					expectInvalidConnection(queryContext)
				})

				Describe("ExecContext", func() {
					expectInvalidConnection(execContext)
				})

				Describe("BeginTx", func() {
					expectInvalidConnection(beginTx)
				})
			})

			Context("when the context deadline is smaller than the connection string timeouts", func() {
				BeforeEach(func() {
					ctx, _ = context.WithTimeout(context.Background(), 500*time.Millisecond)
				})
				Describe("QueryRowContext", func() {
					expectContextDeadlineExceeded(queryRowContext)
				})

				Describe("QueryContext", func() {
					expectContextDeadlineExceeded(queryContext)
				})

				Describe("ExecContext", func() {
					expectContextDeadlineExceeded(execContext)
				})

				Describe("BeginTx", func() {
					expectContextDeadlineExceeded(beginTx)
				})
			})

			Context("when the non-context methods are used", func() {
				Describe("QueryRow", func() {
					expectInvalidConnection(queryRow)
				})

				Describe("Query", func() {
					expectInvalidConnection(query)
				})

				Describe("Exec", func() {
					expectInvalidConnection(exec)
				})

				Describe("Begin", func() {
					expectInvalidConnection(begin)
				})
			})
		})
	})
})

func mustSucceed(binary string, args ...string) string {
	cmd := exec.Command(binary, args...)
	sess, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
	Expect(err).NotTo(HaveOccurred())
	Eventually(sess, "5s").Should(gexec.Exit(0))
	return string(sess.Out.Contents())
}
