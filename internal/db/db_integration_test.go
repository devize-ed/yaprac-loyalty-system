package db

// import (
// 	"context"
// 	"fmt"
// 	"log"
// 	"os"
// 	"testing"
// 	"time"

// 	models "loyaltySys/internal/models"

// 	"github.com/jackc/pgx/v5"
// 	"github.com/ory/dockertest/v3"
// 	"github.com/ory/dockertest/v3/docker"
// 	"github.com/stretchr/testify/assert"
// 	"go.uber.org/zap"
// )

// func TestMain(m *testing.M) {
// 	code, err := runMain(m)
// 	if err != nil {
// 		log.Fatal(err)
// 	}
// 	os.Exit(code)
// }

// const (
// 	testDBName       = "test"
// 	testUserName     = "test"
// 	testUserPassword = "test"
// )

// var (
// 	getDSN          func() string
// 	getSUConnection func() (*pgx.Conn, error)
// )

// func initGetDSN(hostAndPort string) {
// 	getDSN = func() string {
// 		return fmt.Sprintf(
// 			"postgres://%s:%s@%s/%s?sslmode=disable",
// 			testUserName,
// 			testUserPassword,
// 			hostAndPort,
// 			testDBName,
// 		)
// 	}
// }

// func initGetSUConnection() error {
// 	getSUConnection = func() (*pgx.Conn, error) {
// 		conn, err := pgx.Connect(context.Background(), getDSN())
// 		if err != nil {
// 			return nil, fmt.Errorf("failed to get a super user connection: %w", err)
// 		}
// 		return conn, nil
// 	}
// 	return nil
// }

// func runMain(m *testing.M) (int, error) {
// 	pool, err := dockertest.NewPool("")
// 	if err != nil {
// 		return 1, fmt.Errorf("failed to initialize a pool: %w", err)
// 	}

// 	pg, err := pool.RunWithOptions(
// 		&dockertest.RunOptions{
// 			Repository: "postgres",
// 			Tag:        "17.2",
// 			Name:       "migrations-integration-tests",
// 			Env: []string{
// 				"POSTGRES_USER=postgres",
// 				"POSTGRES_PASSWORD=postgres",
// 			},
// 			ExposedPorts: []string{"5432/tcp"},
// 		},
// 		func(config *docker.HostConfig) {
// 			config.AutoRemove = true
// 			config.RestartPolicy = docker.RestartPolicy{Name: "no"}
// 		},
// 	)
// 	if err != nil {
// 		return 1, fmt.Errorf("failed to run the postgres container: %w", err)
// 	}

// 	defer func() {
// 		if err := pool.Purge(pg); err != nil {
// 			log.Printf("failed to purge the postgres container: %v", err)
// 		}
// 	}()

// 	hostPort := pg.GetHostPort("5432/tcp")
// 	initGetDSN(hostPort)
// 	if err := initGetSUConnection(); err != nil {
// 		return 1, err
// 	}

// 	pool.MaxWait = 10 * time.Second
// 	var conn *pgx.Conn
// 	if err := pool.Retry(func() error {
// 		conn, err = getSUConnection()
// 		if err != nil {
// 			return fmt.Errorf("failed to connect to the DB: %w", err)
// 		}
// 		return nil
// 	}); err != nil {
// 		return 1, err
// 	}

// 	defer func() {
// 		if err := conn.Close(context.Background()); err != nil {
// 			log.Printf("failed to correctly close the connection: %v", err)
// 		}
// 	}()

// 	if err := createTestDB(conn); err != nil {
// 		return 1, fmt.Errorf("failed to create a test DB: %w", err)
// 	}

// 	exitCode := m.Run()

// 	return exitCode, nil
// }

// func createTestDB(conn *pgx.Conn) error {
// 	_, err := conn.Exec(context.Background(),
// 		fmt.Sprintf(
// 			`CREATE USER %s PASSWORD '%s'`,
// 			testUserName,
// 			testUserPassword,
// 		),
// 	)
// 	if err != nil {
// 		return fmt.Errorf("failed to create a test user: %w", err)
// 	}

// 	_, err = conn.Exec(context.Background(),
// 		fmt.Sprintf(`
// 			CREATE DATABASE %s
// 				OWNER '%s'
// 				ENCODING 'UTF8'
// 				LC_COLLATE = 'en_US.utf8'
// 				LC_CTYPE = 'en_US.utf8'
// 			`, testDBName, testUserName,
// 		),
// 	)

// 	if err != nil {
// 		return fmt.Errorf("failed to create a test DB: %w", err)
// 	}

// 	return nil
// }

// func Test_CreateUser(t *testing.T) {
// 	testUser := &models.User{
// 		Login:    "test_user",
// 		Password: "test_password",
// 	}

// 	cases := []struct {
// 		Name        string
// 		InUser      *models.User
// 		wantErr     bool
// 		ExpectedErr string
// 	}{
// 		{
// 			Name:        "create_user",
// 			InUser:      testUser,
// 			wantErr:     false,
// 			ExpectedErr: "",
// 		},
// 		{
// 			Name: "create_user_with_empty_password",
// 			InUser: &models.User{
// 				Login:    "test_user",
// 				Password: "",
// 			},
// 			wantErr:     true,
// 			ExpectedErr: "null value in column \"password\"",
// 		},
// 	}

// 	db, err := NewDB(context.Background(), getDSN(), zap.NewNop().Sugar())
// 	if err != nil {
// 		t.Fatalf("failed to create a DB: %v", err)
// 	}
// 	defer db.Close()

// 	for i, tc := range cases {
// 		i, tc := i, tc

// 		t.Run(fmt.Sprintf("test #%d: %s", i, tc.Name), func(t *testing.T) {
// 			userID, err := db.CreateUser(context.Background(), tc.InUser)
// 			if tc.wantErr {
// 				assert.Error(t, err)
// 				assert.ErrorContains(t, err, tc.ExpectedErr)
// 			} else {
// 				assert.NoError(t, err)
// 				assert.NotEqual(t, userID, -1)
// 			}
// 		})
// 	}
// }

// func Test_GetUser(t *testing.T) {
// 	testUser := &models.User{
// 		Login:    "test_user",
// 		Password: "test_password",
// 	}

// 	cases := []struct {
// 		Name        string
// 		InUser      *models.User
// 		wantErr     bool
// 		ExpectedErr string
// 	}{
// 		{
// 			Name:        "get_user",
// 			InUser:      testUser,
// 			wantErr:     false,
// 			ExpectedErr: "",
// 		},
// 		{
// 			Name: "get_unexisting_user",
// 			InUser: &models.User{
// 				Login:    "unexisting_user",
// 				Password: "test_password",
// 			},
// 			wantErr:     true,
// 			ExpectedErr: "not found",
// 		},
// 	}

// 	db, err := NewDB(context.Background(), getDSN(), zap.NewNop().Sugar())
// 	if err != nil {
// 		t.Fatalf("failed to create a DB: %v", err)
// 	}
// 	defer db.Close()

// 	for i, tc := range cases {
// 		i, tc := i, tc

// 		t.Run(fmt.Sprintf("test #%d: %s", i, tc.Name), func(t *testing.T) {
// 			user, err := db.GetUser(context.Background(), tc.InUser.Login)
// 			if tc.wantErr {
// 				assert.Error(t, err)
// 				assert.ErrorContains(t, err, tc.ExpectedErr)
// 			} else {
// 				assert.NoError(t, err)
// 				assert.Equal(t, tc.InUser, user)
// 			}
// 		})
// 	}
// }

// func Test_CreateOrder(t *testing.T) {
// 	cases := []struct {
// 		Name        string
// 		InOrder     *models.Order
// 		wantErr     bool
// 		ExpectedErr string
// 	}{
// 		{
// 			Name: "create_order",
// 			InOrder: &models.Order{
// 				Number:  "1234567890",
// 				Status:  "NEW",
// 				Accrual: 100,
// 			},
// 			wantErr:     false,
// 			ExpectedErr: "",
// 		},
// 		{
// 			Name: "create_order_with_empty_number",
// 			InOrder: &models.Order{
// 				Number:  "unexistingOrder",
// 				Status:  "NEW",
// 				Accrual: 100,
// 			},
// 			wantErr:     true,
// 			ExpectedErr: "null value in column \"number\"",
// 		},
// 	}

// 	db, err := NewDB(context.Background(), getDSN(), zap.NewNop().Sugar())
// 	if err != nil {
// 		t.Fatalf("failed to create a DB: %v", err)
// 	}
// 	defer db.Close()

// 	for i, tc := range cases {
// 		i, tc := i, tc

// 		t.Run(fmt.Sprintf("test #%d: %s", i, tc.Name), func(t *testing.T) {
// 			err := db.CreateOrder(context.Background(), tc.InOrder)
// 			if tc.wantErr {
// 				assert.Error(t, err)
// 				assert.ErrorContains(t, err, tc.ExpectedErr)
// 			} else {
// 				assert.NoError(t, err)
// 			}
// 		})
// 	}
// }

// func Test_GetOrders(t *testing.T) {
// 	testUser := &models.User{
// 		Login:    "test_user",
// 		Password: "test_password",
// 	}

// 	cases := []struct {
// 		Name           string
// 		InUser         *models.User
// 		ExpectedOrders []*models.Order
// 		wantErr        bool
// 		ExpectedErr    string
// 	}{
// 		{
// 			Name:   "get_orders",
// 			InUser: testUser,
// 			ExpectedOrders: []*models.Order{
// 				{
// 					Number:  "1234567890",
// 					Status:  "NEW",
// 					Accrual: 100,
// 				},
// 			},
// 			wantErr:     false,
// 			ExpectedErr: "",
// 		},
// 		{
// 			Name: "get_unexisting_user",
// 			InUser: &models.User{
// 				Login:    "unexisting_user",
// 				Password: "test_password",
// 			},
// 			wantErr:     true,
// 			ExpectedErr: "not found",
// 		},
// 	}

// 	db, err := NewDB(context.Background(), getDSN(), zap.NewNop().Sugar())
// 	if err != nil {
// 		t.Fatalf("failed to create a DB: %v", err)
// 	}
// 	defer db.Close()

// 	for i, tc := range cases {
// 		i, tc := i, tc

// 		t.Run(fmt.Sprintf("test #%d: %s", i, tc.Name), func(t *testing.T) {
// 			orders, err := db.GetOrders(context.Background(), tc.InUser.ID)
// 			if tc.wantErr {
// 				assert.Error(t, err)
// 				assert.ErrorContains(t, err, tc.ExpectedErr)
// 			} else {
// 				assert.NoError(t, err)
// 				assert.Equal(t, tc.ExpectedOrders, orders)
// 			}
// 		})
// 	}
// }

// func Test_GetBalance(t *testing.T) {
// 	testUser := &models.User{
// 		Login:    "test_user",
// 		Password: "test_password",
// 	}

// 	cases := []struct {
// 		Name            string
// 		InUser          *models.User
// 		ExpectedBalance *models.Balance
// 		wantErr         bool
// 		ExpectedErr     string
// 	}{
// 		{
// 			Name:   "get_balance",
// 			InUser: testUser,
// 			ExpectedBalance: &models.Balance{
// 				Current:   100,
// 				Withdrawn: 50,
// 			},
// 			wantErr:     false,
// 			ExpectedErr: "",
// 		},
// 		{
// 			Name: "get_unexisting_user",
// 			InUser: &models.User{
// 				Login:    "unexisting_user",
// 				Password: "test_password",
// 			},
// 			wantErr:     true,
// 			ExpectedErr: "not found",
// 		},
// 	}

// 	db, err := NewDB(context.Background(), getDSN(), zap.NewNop().Sugar())
// 	if err != nil {
// 		t.Fatalf("failed to create a DB: %v", err)
// 	}
// 	defer db.Close()

// 	for i, tc := range cases {
// 		i, tc := i, tc

// 		t.Run(fmt.Sprintf("test #%d: %s", i, tc.Name), func(t *testing.T) {
// 			balance, err := db.GetBalance(context.Background(), tc.InUser.ID)
// 			if tc.wantErr {
// 				assert.Error(t, err)
// 				assert.ErrorContains(t, err, tc.ExpectedErr)
// 			} else {
// 				assert.NoError(t, err)
// 				assert.Equal(t, tc.ExpectedBalance, balance)
// 			}
// 		})
// 	}
// }

// func Test_Withdraw(t *testing.T) {
// 	testUser := &models.User{
// 		Login:    "test_user",
// 		Password: "test_password",
// 	}

// 	cases := []struct {
// 		Name             string
// 		InUser           *models.User
// 		ExpectedWithdraw *models.Withdrawal
// 		wantErr          bool
// 		ExpectedErr      string
// 	}{
// 		{
// 			Name:   "withdraw",
// 			InUser: testUser,
// 			ExpectedWithdraw: &models.Withdrawal{
// 				Order: "1234567890",
// 				Sum:   100,
// 			},
// 			wantErr:     false,
// 			ExpectedErr: "",
// 		},
// 		{
// 			Name:   "withdraw_with_empty_order",
// 			InUser: testUser,
// 			ExpectedWithdraw: &models.Withdrawal{
// 				Order: "unexisting_order",
// 				Sum:   100,
// 			},
// 			wantErr:     true,
// 			ExpectedErr: "null value in column \"order\"",
// 		},
// 		{
// 			Name:   "withdraw_with_empty_sum",
// 			InUser: testUser,
// 			ExpectedWithdraw: &models.Withdrawal{
// 				Order: "1234567890",
// 				Sum:   0,
// 			},
// 			wantErr:     true,
// 			ExpectedErr: "null value in column \"sum\"",
// 		},
// 	}

// 	db, err := NewDB(context.Background(), getDSN(), zap.NewNop().Sugar())
// 	if err != nil {
// 		t.Fatalf("failed to create a DB: %v", err)
// 	}
// 	defer db.Close()

// 	for i, tc := range cases {
// 		i, tc := i, tc

// 		t.Run(fmt.Sprintf("test #%d: %s", i, tc.Name), func(t *testing.T) {
// 			err := db.Withdraw(context.Background(), tc.ExpectedWithdraw)
// 			if tc.wantErr {
// 				assert.Error(t, err)
// 				assert.ErrorContains(t, err, tc.ExpectedErr)
// 			} else {
// 				assert.NoError(t, err)
// 			}
// 			balance, err := db.GetBalance(context.Background(), tc.InUser.ID)
// 			assert.NoError(t, err)
// 			assert.Equal(t, tc.ExpectedWithdraw.Sum, balance.Withdrawn)
// 		})
// 	}
// }

// func Test_GetWithdrawals(t *testing.T) {
// 	testUser := &models.User{
// 		Login:    "test_user",
// 		Password: "test_password",
// 	}

// 	cases := []struct {
// 		Name                string
// 		InUser              *models.User
// 		ExpectedWithdrawals []*models.Withdrawal
// 		wantErr             bool
// 		ExpectedErr         string
// 	}{
// 		{
// 			Name:   "get_withdrawals",
// 			InUser: testUser,
// 			ExpectedWithdrawals: []*models.Withdrawal{
// 				{
// 					Order: "1234567890",
// 					Sum:   100,
// 				},
// 			},
// 			wantErr:     false,
// 			ExpectedErr: "",
// 		},
// 		{
// 			Name: "get_unexisting_user",
// 			InUser: &models.User{
// 				Login:    "unexisting_user",
// 				Password: "test_password",
// 			},
// 			wantErr:     true,
// 			ExpectedErr: "not found",
// 		},
// 	}

// 	db, err := NewDB(context.Background(), getDSN(), zap.NewNop().Sugar())
// 	if err != nil {
// 		t.Fatalf("failed to create a DB: %v", err)
// 	}
// 	defer db.Close()

// 	for i, tc := range cases {
// 		i, tc := i, tc

// 		t.Run(fmt.Sprintf("test #%d: %s", i, tc.Name), func(t *testing.T) {
// 			withdrawals, err := db.GetWithdrawals(context.Background(), tc.InUser.ID)
// 			if tc.wantErr {
// 				assert.Error(t, err)
// 				assert.ErrorContains(t, err, tc.ExpectedErr)
// 			} else {
// 				assert.NoError(t, err)
// 				assert.Equal(t, tc.ExpectedWithdrawals, withdrawals)
// 			}
// 		})
// 	}
// }
