//go:build integration_tests
// +build integration_tests

package db

import (
	"context"
	"fmt"
	"log"
	"loyaltySys/internal/models"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx"
	"github.com/ory/dockertest/v3"
	"github.com/ory/dockertest/v3/docker"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestMain(m *testing.M) {
	code, err := runMain(m)
	if err != nil {
		log.Fatal(err)
	}
	os.Exit(code)
}

const (
	testDBName       = "test"
	testUserName     = "test"
	testUserPassword = "test"
)

var (
	getDSN          func() string
	getSUConnection func() (*pgx.Conn, error)
)

func initGetDSN(hostAndPort string) {
	getDSN = func() string {
		return fmt.Sprintf(
			"postgres://%s:%s@%s/%s?sslmode=disable",
			testUserName,
			testUserPassword,
			hostAndPort,
			testDBName,
		)
	}
}

func initGetSUConnection(hostPort string) error {
	host, port, err := getHostPort(hostPort)
	if err != nil {
		return fmt.Errorf("failed to extract the host and port parts from the string %s: %w", hostPort, err)
	}
	getSUConnection = func() (*pgx.Conn, error) {
		conn, err := pgx.Connect(pgx.ConnConfig{
			Host:     host,
			Port:     port,
			Database: "postgres",
			User:     "postgres",
			Password: "postgres",
		})
		if err != nil {
			return nil, fmt.Errorf("failed to get a super user connection: %w", err)
		}
		return conn, nil
	}
	return nil
}

func runMain(m *testing.M) (int, error) {
	pool, err := dockertest.NewPool("")
	if err != nil {
		return 1, fmt.Errorf("failed to initialize a pool: %w", err)
	}

	pg, err := pool.RunWithOptions(
		&dockertest.RunOptions{
			Repository: "postgres",
			Tag:        "17.2",
			Name:       "migrations-integration-tests",
			Env: []string{
				"POSTGRES_USER=postgres",
				"POSTGRES_PASSWORD=postgres",
			},
			ExposedPorts: []string{"5432/tcp"},
		},
		func(config *docker.HostConfig) {
			config.AutoRemove = true
			config.RestartPolicy = docker.RestartPolicy{Name: "no"}
		},
	)
	if err != nil {
		return 1, fmt.Errorf("failed to run the postgres container: %w", err)
	}

	defer func() {
		if err := pool.Purge(pg); err != nil {
			log.Printf("failed to purge the postgres container: %v", err)
		}
	}()

	hostPort := pg.GetHostPort("5432/tcp")
	initGetDSN(hostPort)
	if err := initGetSUConnection(hostPort); err != nil {
		return 1, err
	}

	pool.MaxWait = 10 * time.Second
	var conn *pgx.Conn
	if err := pool.Retry(func() error {
		conn, err = getSUConnection()
		if err != nil {
			return fmt.Errorf("failed to connect to the DB: %w", err)
		}
		return nil
	}); err != nil {
		return 1, err
	}

	defer func() {
		if err := conn.Close(); err != nil {
			log.Printf("failed to correctly close the connection: %v", err)
		}
	}()

	if err := createTestDB(conn); err != nil {
		return 1, fmt.Errorf("failed to create a test DB: %w", err)
	}

	exitCode := m.Run()

	return exitCode, nil
}

func createTestDB(conn *pgx.Conn) error {
	_, err := conn.Exec(
		fmt.Sprintf(
			`CREATE USER %s PASSWORD '%s'`,
			testUserName,
			testUserPassword,
		),
	)
	if err != nil {
		return fmt.Errorf("failed to create a test user: %w", err)
	}

	_, err = conn.Exec(
		fmt.Sprintf(`
			CREATE DATABASE %s
				OWNER '%s'
				ENCODING 'UTF8'
				LC_COLLATE = 'en_US.utf8'
				LC_CTYPE = 'en_US.utf8'
			`, testDBName, testUserName,
		),
	)

	if err != nil {
		return fmt.Errorf("failed to create a test DB: %w", err)
	}

	return nil
}

func getHostPort(hostPort string) (string, uint16, error) {
	hostPortParts := strings.Split(hostPort, ":")
	if len(hostPortParts) != 2 {
		return "", 0, fmt.Errorf("got an invalid host-port string: %s", hostPort)
	}

	portStr := hostPortParts[1]
	port, err := strconv.Atoi(portStr)
	if err != nil {
		return "", 0, fmt.Errorf("failed to cast the port %s to an int: %w", portStr, err)
	}
	return hostPortParts[0], uint16(port), nil
}

func newTestDB(t *testing.T) *DB {
	t.Helper()
	dsn := getDSN()
	db, err := NewDB(context.Background(), dsn, zap.NewNop().Sugar())
	if err != nil {
		t.Error(err)
		return nil
	}
	return db
}

func closeTestDB(t *testing.T, db *DB) {
	t.Helper()
	if err := db.Close(); err != nil {
		t.Error(err)
	}
}

func TestDB_CreateUser(t *testing.T) {
	db := newTestDB(t)
	defer closeTestDB(t, db)

	cases := []struct {
		Name        string
		InUser      *models.User
		wantUserID  int64
		ExpectedErr error
		wantErr     bool
	}{
		{
			Name: "add_user",
			InUser: &models.User{
				Login:    "test1",
				Password: "test1",
			},
			wantUserID:  1,
			ExpectedErr: nil,
			wantErr:     false,
		},
		{
			Name: "add_user_already_exists",
			InUser: &models.User{
				Login:    "test1",
				Password: "test1",
			},
			wantUserID:  -1,
			ExpectedErr: ErrUserAlreadyExists,
			wantErr:     true,
		},
	}

	for i, tc := range cases {
		i, tc := i, tc

		t.Run(fmt.Sprintf("test #%d: %s", i, tc.Name), func(t *testing.T) {
			userID, err := db.CreateUser(context.Background(), tc.InUser)
			if tc.wantErr {
				assert.Error(t, err)
				assert.Equal(t, tc.ExpectedErr, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tc.wantUserID, userID)
			}
		})
	}
}

func TestDB_GetUser(t *testing.T) {
	db := newTestDB(t)
	defer closeTestDB(t, db)

	cases := []struct {
		Name        string
		User        *models.User
		wantUser    *models.User
		ExpectedErr error
		wantErr     bool
	}{
		{
			Name: "get_user",
			User: &models.User{
				Login: "test1",
			},
			wantUser: &models.User{
				ID:       1,
				Login:    "test1",
				Password: "test1",
			},
			ExpectedErr: nil,
			wantErr:     false,
		},
		{
			Name: "user_not_found",
			User: &models.User{
				Login: "test2",
			},
			wantUser:    nil,
			ExpectedErr: ErrUserNotFound,
			wantErr:     true,
		},
	}

	for i, tc := range cases {
		i, tc := i, tc

		t.Run(fmt.Sprintf("test #%d: %s", i, tc.Name), func(t *testing.T) {
			user, err := db.GetUser(context.Background(), tc.User.Login)
			if tc.wantErr {
				assert.Error(t, err)
				assert.Equal(t, tc.ExpectedErr, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tc.wantUser, user)
			}
		})
	}
}

func TestDB_CreateOrder(t *testing.T) {
	db := newTestDB(t)
	defer closeTestDB(t, db)

	cases := []struct {
		Name        string
		Order       *models.Order
		ExpectedErr error
		wantErr     bool
	}{
		{
			Name: "create_order",
			Order: &models.Order{
				Number: "1234567890",
				UserID: 1,
			},
		},
		{
			Name: "create_order_already_exists",
			Order: &models.Order{
				Number: "1234567890",
				UserID: 1,
			},
			ExpectedErr: ErrOrderAlreadyExists,
			wantErr:     true,
		},
		{
			Name: "create_order_already_added_by_another_user",
			Order: &models.Order{
				Number: "1234567890",
				UserID: 2,
			},
			ExpectedErr: ErrOrderAlreadyAdded,
			wantErr:     true,
		},
	}
	for i, tc := range cases {
		i, tc := i, tc
		t.Run(fmt.Sprintf("test #%d: %s", i, tc.Name), func(t *testing.T) {
			err := db.CreateOrder(context.Background(), tc.Order)
			if tc.wantErr {
				assert.Error(t, err)
				assert.Equal(t, tc.ExpectedErr, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestDB_GetOrders(t *testing.T) {
	db := newTestDB(t)
	defer closeTestDB(t, db)

	cases := []struct {
		Name   string
		UserID int64
		want   *models.Order
	}{
		{
			Name:   "get_orders",
			UserID: 1,
			want: &models.Order{
				Number: "1234567890",
				Status: "NEW",
			},
		},
		{
			Name:   "get_orders_not_found",
			UserID: 2,
			want:   nil,
		},
	}
	for i, tc := range cases {
		i, tc := i, tc
		t.Run(fmt.Sprintf("test #%d: %s", i, tc.Name), func(t *testing.T) {
			orders, err := db.GetOrders(context.Background(), tc.UserID)
			assert.NoError(t, err)
			if tc.want == nil {
				require.Empty(t, orders)
			} else {
				require.NotEmpty(t, orders)
				assert.Equal(t, tc.want.Number, orders[0].Number)
				assert.Equal(t, tc.want.Status, orders[0].Status)
				assert.NotEmpty(t, orders[0].UploadedAt)
			}
		})
	}
}

func TestDB_GetUnprocessedOrders(t *testing.T) {
	db := newTestDB(t)
	defer closeTestDB(t, db)

	cases := []struct {
		Name string
		want []*models.Order
	}{
		{
			Name: "get_unprocessed_orders",
			want: []*models.Order{
				{
					Number: "1234567890",
					Status: "NEW",
				},
			},
		},
	}

	for i, tc := range cases {
		i, tc := i, tc
		t.Run(fmt.Sprintf("test #%d: %s", i, tc.Name), func(t *testing.T) {
			orders, err := db.GetUnprocessedOrders(context.Background())
			assert.NoError(t, err)
			if tc.want == nil {
				require.Empty(t, orders)
			} else {
				require.NotEmpty(t, orders)
				assert.Equal(t, tc.want[0].Number, orders[0].Number)
				assert.Equal(t, tc.want[0].Status, orders[0].Status)
				assert.NotEmpty(t, orders[0].UploadedAt)
			}
		})
	}
}

func TestDB_UpdateOrder(t *testing.T) {
	db := newTestDB(t)
	defer closeTestDB(t, db)

	cases := []struct {
		Name        string
		Order       *models.Order
		ExpectedErr error
		wantErr     bool
	}{
		{
			Name: "update_order",
			Order: &models.Order{
				Number:  "1234567890",
				Status:  "PROCESSED",
				Accrual: 100,
			},
			ExpectedErr: nil,
			wantErr:     false,
		},
		{
			Name: "update_order_not_found",
			Order: &models.Order{
				Number:  "1234567891",
				Status:  "PROCESSED",
				Accrual: 100,
			},
			ExpectedErr: ErrOrderNotFound,
			wantErr:     true,
		},
	}
	for i, tc := range cases {
		i, tc := i, tc
		t.Run(fmt.Sprintf("test #%d: %s", i, tc.Name), func(t *testing.T) {
			err := db.UpdateOrder(context.Background(), tc.Order)
			if tc.wantErr {
				assert.Error(t, err)
				assert.Equal(t, tc.ExpectedErr, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestDB_Withdraw(t *testing.T) {
	db := newTestDB(t)
	defer closeTestDB(t, db)

	cases := []struct {
		Name        string
		Withdrawal  *models.Withdrawal
		ExpectedErr error
		wantErr     bool
	}{
		{
			Name: "withdraw",
			Withdrawal: &models.Withdrawal{
				UserID: 1,
				Order:  "1234567890",
				Sum:    20,
			},
			ExpectedErr: nil,
			wantErr:     false,
		},
		{
			Name: "insufficient_balance",
			Withdrawal: &models.Withdrawal{
				UserID: 1,
				Order:  "1234567891",
				Sum:    250,
			},
			ExpectedErr: ErrInsufficientBalance,
			wantErr:     true,
		},
	}
	for i, tc := range cases {
		i, tc := i, tc
		t.Run(fmt.Sprintf("test #%d: %s", i, tc.Name), func(t *testing.T) {
			err := db.Withdraw(context.Background(), tc.Withdrawal)
			if tc.wantErr {
				assert.Error(t, err)
				assert.Equal(t, tc.ExpectedErr, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}

}

func TestDB_GetWithdrawals(t *testing.T) {
	db := newTestDB(t)
	defer closeTestDB(t, db)

	cases := []struct {
		Name   string
		UserID int64
		want   *models.Withdrawal
	}{
		{
			Name:   "get_orders",
			UserID: 1,
			want: &models.Withdrawal{
				Order:       "1234567890",
				Sum:         20,
				ProcessedAt: time.Now(),
			},
		},
		{
			Name:   "get_orders_not_found",
			UserID: 2,
			want:   nil,
		},
	}
	for i, tc := range cases {
		i, tc := i, tc
		t.Run(fmt.Sprintf("test #%d: %s", i, tc.Name), func(t *testing.T) {
			withdrawals, err := db.GetWithdrawals(context.Background(), tc.UserID)
			assert.NoError(t, err)
			if tc.want == nil {
				require.Empty(t, withdrawals)
			} else {
				require.NotEmpty(t, withdrawals)
				assert.Equal(t, tc.want.Order, withdrawals[0].Order)
				assert.Equal(t, tc.want.Sum, withdrawals[0].Sum)
				assert.NotEmpty(t, withdrawals[0].ProcessedAt)
			}
		})
	}
}

func TestDB_GetBalance(t *testing.T) {
	db := newTestDB(t)
	defer closeTestDB(t, db)

	cases := []struct {
		Name   string
		UserID int64
		want   *models.Balance
	}{
		{
			Name:   "get_balance",
			UserID: 1,
			want: &models.Balance{
				Current:   80,
				Withdrawn: 20,
			},
		},
	}
	for i, tc := range cases {
		i, tc := i, tc
		t.Run(fmt.Sprintf("test #%d: %s", i, tc.Name), func(t *testing.T) {
			balance, err := db.GetBalance(context.Background(), tc.UserID)
			assert.NoError(t, err)
			assert.Equal(t, tc.want, balance)
		})
	}
}
