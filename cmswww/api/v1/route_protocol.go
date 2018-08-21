package v1

import (
	"fmt"
)

const (
	RouteRoot              = "/"
	RouteInviteNewUser     = "/user/invite"
	RouteRegister          = "/user/new"
	RouteNewIdentity       = "/user/identity"
	RouteVerifyNewIdentity = "/user/identity/verify"
	RouteUserInvoices      = "/user/invoices"
	RouteUserDetails       = "/user"
	RouteEditUser          = "/user/edit"
	RouteLogin             = "/login"
	RouteLogout            = "/logout"
	RouteInvoices          = "/invoices"
	RouteSubmitInvoice     = "/invoice/submit"
	RouteInvoiceDetails    = "/invoice"
	RouteSetInvoiceStatus  = "/invoice/setstatus"
	RoutePolicy            = "/policy"
)

var (
	// APIRoute is the prefix to the API route
	APIRoute = fmt.Sprintf("/v%v", APIVersion)
)

// File describes an individual file that is part of the invoice.  The
// directory structure must be flattened.  The server side SHALL verify MIME
// and Digest.
type File struct {
	// Meta-data
	Name   string `json:"name"`   // Suggested filename
	MIME   string `json:"mime"`   // Mime type
	Digest string `json:"digest"` // Digest of unencoded payload

	// Data
	Payload string `json:"payload"` // File content, base64 encoded
}

// CensorshipRecord contains the proof that an invoice was accepted for review.
// The proof is verifiable on the client side.
//
// The Merkle field contains the digest of the invoice file.
// The Token field contains a random censorship token that is signed by the
// server private key.  The token can be used on the client to verify the
// authenticity of the CensorshipRecord.
type CensorshipRecord struct {
	Token     string `json:"token"`     // Censorship token
	Merkle    string `json:"merkle"`    // Digest of invoice file
	Signature string `json:"signature"` // Server side signature of []byte(Merkle+Token)
}

// InvoiceRecord is an entire invoice and its content.
type InvoiceRecord struct {
	Status    InvoiceStatusT `json:"status"`    // Current status of invoice
	Timestamp int64          `json:"timestamp"` // Last update of invoice
	Month     uint16         `json:"month"`     // The month that this invoice applies to
	Year      uint16         `json:"year"`      // The year that this invoice applies to
	UserID    string         `json:"userid"`    // ID of user who submitted invoice
	Username  string         `json:"username"`  // Username of user who submitted invoice
	PublicKey string         `json:"publickey"` // User's public key, used to verify signature.
	Signature string         `json:"signature"` // Signature of file digest
	File      *File          `json:"file"`      // Actual invoice file

	CensorshipRecord CensorshipRecord `json:"censorshiprecord"`
}

// UserError represents an error that is caused by something that the user
// did (malformed input, bad timing, etc).
type UserError struct {
	ErrorCode    ErrorStatusT
	ErrorContext []string
}

// Error satisfies the error interface.
func (e UserError) Error() string {
	return fmt.Sprintf("user error code: %v", e.ErrorCode)
}

// PDError is emitted when an HTTP error response is returned from Politeiad
// for a request. It contains the HTTP status code and the JSON response body.
type PDError struct {
	HTTPCode   int
	ErrorReply PDErrorReply
}

// Error satisfies the error interface.
func (e PDError) Error() string {
	return fmt.Sprintf("error from politeiad: %v %v", e.HTTPCode,
		e.ErrorReply.ErrorCode)
}

// PDErrorReply is an error reply returned from Politeiad whenever an
// error occurs.
type PDErrorReply struct {
	ErrorCode    int
	ErrorContext []string
}

// ErrorReply are replies that the server returns a when it encounters an
// unrecoverable problem while executing a command.  The HTTP Error Code
// shall be 500 if it's an internal server error or 4xx if it's a user error.
type ErrorReply struct {
	ErrorCode    int64    `json:"errorcode,omitempty"`
	ErrorContext []string `json:"errorcontext,omitempty"`
}

// Version command is used to determine the version of the API this backend
// understands and additionally it provides the route to said API.  This call
// is required in order to establish CSRF for the session.  The client should
// verify compatibility with the server version.
type Version struct{}

// VersionReply returns information that indicates what version of the server
// is running and additionally the route to the API and the public signing key of
// the server.
type VersionReply struct {
	Version   uint        `json:"version"`        // politeia WWW API version
	Route     string      `json:"route"`          // prefix to API calls
	PublicKey string      `json:"publickey"`      // Server public key
	TestNet   bool        `json:"testnet"`        // Network indicator
	User      *LoginReply `json:"user,omitempty"` // Currently logged in user
}

// InviteNewUser is used to request that a new user invitation be sent via email.
// If successful, the user will require verification before being able to login.
type InviteNewUser struct {
	Email string `json:"email"`
}

// InviteNewUserReply responds with the verification token for the user
// (if an email server is not set up).
type InviteNewUserReply struct {
	VerificationToken string `json:"verificationtoken"`
}

// Register is used to request that a new user be verified.
type Register struct {
	Email             string `json:"email"`
	Username          string `json:"username"`
	Password          string `json:"password"`
	VerificationToken string `json:"verificationtoken"`
	PublicKey         string `json:"publickey"`
	Signature         string `json:"signature"`
}

// RegisterReply replies to Register with no properties, if successful.
type RegisterReply struct{}

// NewIdentity is used to generate a new identity.
type NewIdentity struct {
	PublicKey string `json:"publickey"`
}

// NewIdentityReply replies to the NewIdentity command.
type NewIdentityReply struct {
	VerificationToken string `json:"verificationtoken"` // Server verification token
}

// VerifyNewIdentity is used to verify the generation of a new identitys.
type VerifyNewIdentity struct {
	VerificationToken string `json:"verificationtoken"` // Server provided verification token
	Signature         string `json:"signature"`         // Verification token signature
}

// VerifyNewIdentityReply replies to the VerifyNewIdentity command.
type VerifyNewIdentityReply struct{}

// UserInvoices is used to request a list of invoices that the
// user has submitted.
type UserInvoices struct {
	UserID string `schema:"userid"`
}

// UserInvoicesReply replies to the UserInvoices command with
// a list of invoices that the user has submitted.
type UserInvoicesReply struct {
	Invoices []InvoiceRecord `json:"invoices"`
}

// Login attempts to login the user.  Note that by necessity the password
// travels in the clear.
type Login struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// LoginReply is used to reply to the Login command.
type LoginReply struct {
	IsAdmin   bool   `json:"isadmin"`   // Set if user is an admin
	UserID    string `json:"userid"`    // User id
	Email     string `json:"email"`     // User email
	Username  string `json:"username"`  // Username
	PublicKey string `json:"publickey"` // Active public key
	LastLogin int64  `json:"lastlogin"` // Unix timestamp of last login date
}

// Logout attempts to log the user out.
type Logout struct{}

// LogoutReply indicates whether the Logout command was success or not.
type LogoutReply struct{}

// SubmitInvoice attempts to submit a new invoice.
type SubmitInvoice struct {
	Month     uint16 `json:"month"`
	Year      uint16 `json:"year"`
	File      File   `json:"file"`      // Invoice file
	PublicKey string `json:"publickey"` // Key used to verify signature
	Signature string `json:"signature"` // Signature of file hash
}

// SubmitInvoiceReply is used to reply to the SubmitInvoice command.
type SubmitInvoiceReply struct {
	CensorshipRecord CensorshipRecord `json:"censorshiprecord"`
}

// InvoiceDetails is used to retrieve an invoice.
type InvoiceDetails struct {
	Token string `json:"token"`
}

// InvoiceDetailsReply is used to reply to an invoice details command.
type InvoiceDetailsReply struct {
	Invoice InvoiceRecord `json:"invoice"`
}

// SetInvoiceStatus is used to publish or censor an unreviewed invoice.
type SetInvoiceStatus struct {
	Token     string         `json:"token"`
	Status    InvoiceStatusT `json:"status"`
	Signature string         `json:"signature"` // Signature of Token+string(InvoiceStatus)
	PublicKey string         `json:"publickey"` // Public key of admin
}

// SetInvoiceStatusReply is used to reply to a SetInvoiceStatus command.
type SetInvoiceStatusReply struct {
	Invoice InvoiceRecord `json:"invoice"`
}

// Invoices retrieves all invoices with a given status for a given month & year.
//
// Note: This call requires admin privileges.
type Invoices struct {
	Status InvoiceStatusT `json:"status"`
	Month  uint16         `json:"month"`
	Year   uint16         `json:"year"`
}

// InvoicesReply is used to reply with a list of invoices.
type InvoicesReply struct {
	Invoices []InvoiceRecord `json:"invoices"`
}

// MyInvoices retrieves all invoices with a given status for a user.
type MyInvoices struct {
	Status InvoiceStatusT `json:"status"`
}

// MyInvoicesReply is used to reply with a list of user's invoices.
type MyInvoicesReply struct {
	Invoices []InvoiceRecord `json:"invoices"`
}

// Policy returns a struct with various maxima.  The client shall observe the
// maxima.
type Policy struct{}

// PolicyReply is used to reply to the policy command. It returns
// the file upload restrictions set for Politeia.
type PolicyReply struct {
	MinPasswordLength      uint          `json:"minpasswordlength"`
	MinUsernameLength      uint          `json:"minusernamelength"`
	MaxUsernameLength      uint          `json:"maxusernamelength"`
	UsernameSupportedChars []string      `json:"usernamesupportedchars"`
	ListPageSize           uint          `json:"listpagesize"`
	ValidMIMETypes         []string      `json:"validmimetypes"`
	Invoice                InvoicePolicy `json:"invoice"`
}

// InvoicePolicy is the specific policy related to invoice submission.
type InvoicePolicy struct {
	FieldDelimiterChar rune `json:"fielddelimiterchar"`
	CommentChar        rune `json:"commentchar"`
}

// UserDetails fetches a user's details by their id, email, or username.
type UserDetails struct {
	UserID   string `json:"userid"`
	Email    string `json:"email"`
	Username string `json:"username"`
}

// UserDetailsReply returns a user's details.
type UserDetailsReply struct {
	User User `json:"user"`
}

// EditUser performs the given action on a user given their id, email or username.
type EditUser struct {
	UserID   string          `json:"userid"`
	Email    string          `json:"email"`
	Username string          `json:"username"`
	Action   UserEditActionT `json:"action"` // Action
	Reason   string          `json:"reason"` // Admin reason for action
}

// EditUserReply is the reply for the EditUserReply command.
type EditUserReply struct {
	VerificationToken *string `json:"verificationtoken"` // Only set for certain user edit actions
}

// User represents an individual user.
type User struct {
	ID                               string          `json:"id"`
	Email                            string          `json:"email"`
	Username                         string          `json:"username"`
	Admin                            bool            `json:"isadmin"`
	RegisterVerificationToken        []byte          `json:"newuserverificationtoken"`
	RegisterVerificationExpiry       int64           `json:"newuserverificationexpiry"`
	UpdateIdentityVerificationToken  []byte          `json:"updateidentityverificationtoken"`
	UpdateIdentityVerificationExpiry int64           `json:"updateidentityverificationexpiry"`
	LastLogin                        int64           `json:"lastlogin"`
	FailedLoginAttempts              uint64          `json:"failedloginattempts"`
	Locked                           bool            `json:"islocked"`
	Identities                       []UserIdentity  `json:"identities"`
	Invoices                         []InvoiceRecord `json:"invoices"`
}

// UserIdentity represents a user's unique identity.
type UserIdentity struct {
	PublicKey string `json:"publickey"`
	Active    bool   `json:"isactive"`
}
