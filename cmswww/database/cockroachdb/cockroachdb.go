package cockroachdb

import (
	"fmt"
	"math/rand"
	"net/url"
	"path/filepath"
	"strconv"
	"sync"

	"github.com/badoux/checkmail"
	"github.com/jinzhu/gorm"
	_ "github.com/jinzhu/gorm/dialects/postgres"

	"github.com/decred/contractor-mgmt/cmswww/database"
)

var (
	_ database.Database = (*cockroachdb)(nil)
)

// cockroachdb implements the database interface.
type cockroachdb struct {
	sync.RWMutex
	shutdown bool // Backend is shutdown
	db       *gorm.DB
}

// Version contains the database version.
type Version struct {
	Version uint32 `json:"version"` // Database version
	Time    int64  `json:"time"`    // Time of record creation
}

func (c *cockroachdb) addWhereClause(db *gorm.DB, paramsMap map[string]interface{}) *gorm.DB {
	for k, v := range paramsMap {
		db = db.Where(k+"= ?", v)
	}
	return db
}

const letterBytes = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"

func (c *cockroachdb) dropTable(tableName string) error {
	b := make([]byte, 4)
	for i := range b {
		b[i] = letterBytes[rand.Intn(len(letterBytes))]
	}
	newTableName := tableName + string(b)

	result := c.db.Exec(fmt.Sprintf("ALTER TABLE IF EXISTS %v RENAME TO %v;",
		tableName, newTableName))
	if result.Error != nil {
		return result.Error
	}

	result = c.db.Exec(fmt.Sprintf("DROP TABLE IF EXISTS %v CASCADE;",
		newTableName))
	return result.Error
}

// Store new user.
//
// CreateUser satisfies the backend interface.
func (c *cockroachdb) CreateUser(dbUser *database.User) error {
	c.Lock()
	defer c.Unlock()

	if c.shutdown {
		return database.ErrShutdown
	}

	user := EncodeUser(dbUser)
	log.Debugf("NewUser: %v", user.Email)

	if err := checkmail.ValidateFormat(user.Email); err != nil {
		return database.ErrInvalidEmail
	}

	return c.db.Create(user).Error
}

// Update an existing user.
//
// UpdateUser satisfies the backend interface.
func (c *cockroachdb) UpdateUser(dbUser *database.User) error {
	c.Lock()
	defer c.Unlock()

	if c.shutdown {
		return database.ErrShutdown
	}

	user := EncodeUser(dbUser)
	log.Debugf("UpdateUser: %v", user.Email)
	return c.db.Model(&User{}).Updates(*user).Error
}

// GetUser returns a user record if found in the database.
//
// GetUser satisfies the backend interface.
func (c *cockroachdb) GetUserByEmail(email string) (*database.User, error) {
	c.Lock()
	defer c.Unlock()

	if c.shutdown {
		return nil, database.ErrShutdown
	}

	var user User
	result := c.db.Where("email = ?", email).Preload("Identities").First(&user)
	if result.Error != nil {
		if gorm.IsRecordNotFoundError(result.Error) {
			return nil, database.ErrUserNotFound
		}
		return nil, result.Error
	}

	return DecodeUser(&user)
}

// GetUserByUsername returns a user record given its username, if found in the database.
//
// GetUserByUsername satisfies the backend interface.
func (c *cockroachdb) GetUserByUsername(username string) (*database.User, error) {
	c.Lock()
	defer c.Unlock()

	if c.shutdown {
		return nil, database.ErrShutdown
	}

	var user User
	result := c.db.Where("username = ?", username).Preload("Identities").First(&user)
	if result.Error != nil {
		if gorm.IsRecordNotFoundError(result.Error) {
			return nil, database.ErrUserNotFound
		}
		return nil, result.Error
	}

	return DecodeUser(&user)
}

// GetUserById returns a user record given its id, if found in the database.
//
// GetUserById satisfies the backend interface.
func (c *cockroachdb) GetUserById(id uint64) (*database.User, error) {
	c.Lock()
	defer c.Unlock()

	if c.shutdown {
		return nil, database.ErrShutdown
	}

	var user User
	result := c.db.Preload("Identities").First(&user, id)
	if result.Error != nil {
		if gorm.IsRecordNotFoundError(result.Error) {
			return nil, database.ErrUserNotFound
		}
		return nil, result.Error
	}

	return DecodeUser(&user)
}

// GetUserIdByPublicKey returns a user record given its id, if found in the database.
//
// GetUserIdByPublicKey satisfies the backend interface.
func (c *cockroachdb) GetUserIdByPublicKey(publicKey string) (uint64, error) {
	var id Identity
	result := c.db.Where("key = ?", publicKey).First(&id)
	if result.Error != nil {
		return 0, result.Error
	}

	return uint64(id.UserID), nil
}

// Executes a callback on every user in the database.
//
// AllUsers satisfies the backend interface.
func (c *cockroachdb) AllUsers(callbackFn func(u *database.User)) error {
	c.Lock()
	defer c.Unlock()

	if c.shutdown {
		return database.ErrShutdown
	}

	log.Debugf("AllUsers")

	var users []User
	result := c.db.Find(&users)
	if result.Error != nil {
		return result.Error
	}

	for _, user := range users {
		dbUser, err := DecodeUser(&user)
		if err != nil {
			return err
		}

		callbackFn(dbUser)
	}

	return nil
}

// Create new invoice.
//
// CreateInvoice satisfies the backend interface.
func (c *cockroachdb) CreateInvoice(dbInvoice *database.Invoice) error {
	c.Lock()
	defer c.Unlock()

	if c.shutdown {
		return database.ErrShutdown
	}

	invoice := EncodeInvoice(dbInvoice)

	log.Debugf("CreateInvoice: %v", invoice.Token)
	return c.db.Create(invoice).Error
}

// Update existing invoice.
//
// CreateInvoice satisfies the backend interface.
func (c *cockroachdb) UpdateInvoice(dbInvoice *database.Invoice) error {
	c.Lock()
	defer c.Unlock()

	if c.shutdown {
		return database.ErrShutdown
	}

	invoice := EncodeInvoice(dbInvoice)

	log.Debugf("UpdateInvoice: %v", invoice.Token)

	return c.db.Save(invoice).Error
}

// Return invoice by its token.
func (c *cockroachdb) GetInvoiceByToken(token string) (*database.Invoice, error) {
	c.Lock()
	defer c.Unlock()

	if c.shutdown {
		return nil, database.ErrShutdown
	}

	log.Debugf("GetInvoiceByToken: %v", token)

	var invoice Invoice
	result := c.db.Table(fmt.Sprintf("%v i", tableNameInvoice)).Select("i.*, u.username").Joins(
		"inner join users u on i.user_id = u.id").Where(
		"i.token = ?", token).Scan(&invoice)
	if result.Error != nil {
		if gorm.IsRecordNotFoundError(result.Error) {
			return nil, database.ErrInvoiceNotFound
		}
		return nil, result.Error
	}

	return DecodeInvoice(&invoice)
}

// Return a list of invoices.
func (c *cockroachdb) GetInvoices(invoicesRequest database.InvoicesRequest) ([]database.Invoice, error) {
	c.Lock()
	defer c.Unlock()

	if c.shutdown {
		return nil, database.ErrShutdown
	}

	log.Debugf("GetInvoices")

	paramsMap := make(map[string]interface{})
	var err error
	if invoicesRequest.UserID != "" {
		paramsMap["i.user_id"], err = strconv.ParseUint(invoicesRequest.UserID, 10, 64)
		if err != nil {
			return nil, err
		}
	}

	if invoicesRequest.StatusMap != nil && len(invoicesRequest.StatusMap) > 0 {
		statuses := make([]uint, 0, len(invoicesRequest.StatusMap))
		for k := range invoicesRequest.StatusMap {
			statuses = append(statuses, uint(k))
		}
		paramsMap["i.status"] = statuses
	}

	if invoicesRequest.Month != 0 {
		paramsMap["i.month"] = invoicesRequest.Month
	}

	if invoicesRequest.Year != 0 {
		paramsMap["i.year"] = invoicesRequest.Year
	}

	var invoices []Invoice
	db := c.db.Table(fmt.Sprintf("%v i", tableNameInvoice)).Select("i.*, u.username").Joins(
		fmt.Sprintf("inner join %v u on i.user_id = u.id", tableNameUser))
	db = c.addWhereClause(db, paramsMap)
	result := db.Scan(&invoices)
	if result.Error != nil {
		if gorm.IsRecordNotFoundError(result.Error) {
			return nil, database.ErrInvoiceNotFound
		}
		return nil, result.Error
	}

	return DecodeInvoices(invoices)
}

// Deletes all data from all tables.
//
// DeleteAllData satisfies the backend interface.
func (c *cockroachdb) DeleteAllData() error {
	c.Lock()
	defer c.Unlock()

	if c.shutdown {
		return database.ErrShutdown
	}

	log.Debugf("DeleteAllData")

	c.dropTable(tableNameInvoicePayment)
	c.dropTable(tableNameInvoiceChange)
	c.dropTable(tableNameInvoice)
	c.dropTable(tableNameIdentity)
	c.dropTable(tableNameUser)
	return nil
}

// Close shuts down the database.  All interface functions MUST return with
// errShutdown if the backend is shutting down.
//
// Close satisfies the backend interface.
func (c *cockroachdb) Close() error {
	c.Lock()
	defer c.Unlock()

	c.shutdown = true
	return c.db.Close()
}

// New creates a new cockroachdb instance.
func New(dataDir, dbName, username, host string) (*cockroachdb, error) {
	log.Tracef("cockroachdb New")

	cockroachDBFile := filepath.Join(dataDir, "cockroachdb")

	v := url.Values{}
	v.Set("sslcert", filepath.Join(cockroachDBFile,
		fmt.Sprintf("client.%v.crt", username)))
	v.Set("sslkey", filepath.Join(cockroachDBFile,
		fmt.Sprintf("client.%v.key", username)))
	v.Set("sslmode", "verify-full")
	v.Set("sslrootcert", filepath.Join(cockroachDBFile, "ca.crt"))
	dataSource := fmt.Sprintf("postgresql://%v@%v/%v?%v", username, host,
		dbName, v.Encode())

	db, err := gorm.Open("postgres", dataSource)
	if err != nil {
		return nil, fmt.Errorf("error connecting to the database: %v", err)
	}

	db.LogMode(true)

	c := cockroachdb{
		db: db,
	}

	err = c.dropTable(tableNameInvoice)
	if err != nil {
		return nil, fmt.Errorf("error dropping invoice table: %v", err)
	}
	c.db.AutoMigrate(
		&User{},
		&Identity{},
		&Invoice{},
		&InvoiceChange{},
		&InvoicePayment{},
	)

	return &c, nil
}
