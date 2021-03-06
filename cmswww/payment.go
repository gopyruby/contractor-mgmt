package main

import (
	"fmt"
	"time"

	"github.com/decred/politeia/util"

	"github.com/decred/contractor-mgmt/cmswww/api/v1"
	"github.com/decred/contractor-mgmt/cmswww/database"
)

type polledPayment struct {
	address     string // Payment address
	amount      uint64 // Expected tx amount required to satisfy payment
	txNotBefore int64  // Minimum timestamp for payment tx
	pollExpiry  int64  // After this time, the payment address will not be continuously polled
}

const (
	// pollExpiryDuration is the amount of time the server will watch a payment address
	// for transactions.
	pollExpiryDuration = time.Hour * 24

	// pollCheckGap is the amount of time the server sleeps after polling for
	// a payment address.
	pollCheckGap = time.Second * 5
)

func pollHasExpired(pollExpiry int64) bool {
	return time.Now().After(time.Unix(pollExpiry, 0))
}

func (c *cmswww) derivePaymentInfo(user *database.User) (string, int64, error) {
	address, err := util.DerivePaywallAddress(c.params,
		user.ExtendedPublicKey, uint32(user.PaymentAddressIndex))
	if err != nil {
		err = fmt.Errorf("Unable to derive payment address "+
			"for user %v (%v): %v", user.ID, user.Email, err)
	}

	return address, time.Now().Unix(), err
}

// addInvoiceForPolling adds an invoice's payment info to the in-memory map.
//
// This function must be called WITH the mutex held.
func (c *cmswww) addInvoiceForPolling(token string, invoicePayment *database.InvoicePayment) {
	c.polledPayments[token] = polledPayment{
		address:     invoicePayment.Address,
		amount:      invoicePayment.Amount,
		txNotBefore: invoicePayment.TxNotBefore,
		pollExpiry:  invoicePayment.PollExpiry,
	}
}

// addInvoiceForPollingLock adds an invoice's payment info to the in-memory map.
//
// This function must be called WITHOUT the mutex held.
func (c *cmswww) addInvoiceForPollingLock(token string, invoicePayment *database.InvoicePayment) {
	c.Lock()
	defer c.Unlock()

	c.addInvoiceForPolling(token, invoicePayment)
}

func (c *cmswww) addInvoicesForPolling() error {
	c.Lock()
	defer c.Unlock()

	// Create the in-memory pool of all users who need to pay the payment.
	invoices, err := c.db.GetInvoices(database.InvoicesRequest{})
	if err != nil {
		return err
	}

	for _, invoice := range invoices {
		for _, invoicePayment := range invoice.Payments {
			if pollHasExpired(invoicePayment.PollExpiry) {
				continue
			}

			c.addInvoiceForPolling(invoice.Token, &invoicePayment)
		}
	}

	log.Tracef("Added %v invoices to payment pool", len(c.polledPayments))
	return nil
}

func (c *cmswww) createPolledPaymentsCopy() map[string]polledPayment {
	c.RLock()
	defer c.RUnlock()

	copy := make(map[string]polledPayment, len(c.polledPayments))

	for k, v := range c.polledPayments {
		copy[k] = v
	}

	return copy
}

func (c *cmswww) checkForInvoicePayments(polledPayments map[string]polledPayment) (bool, []string) {
	var tokensToRemove []string

	for token, polledPayment := range polledPayments {
		invoice, err := c.db.GetInvoiceByToken(token)
		if err != nil {
			if err == database.ErrShutdown {
				// The database is shutdown, so stop the thread.
				return false, nil
			}

			log.Errorf("cannot fetch invoice by token %v: %v\n", token, err)
			continue
		}

		log.Tracef("Checking the payment address for invoice %v...",
			token)

		if invoice.Status == v1.InvoiceStatusPaid {
			// The invoice could have been marked as paid by some external
			// mechanism, so just remove him from polling.
			tokensToRemove = append(tokensToRemove, token)
			log.Tracef("  removing from polling, invoice already paid")
			continue
		}

		if pollHasExpired(polledPayment.pollExpiry) {
			tokensToRemove = append(tokensToRemove, invoice.Token)
			log.Tracef("  removing from polling, poll has expired")
			continue
		}

		tx, _, err := util.FetchTxWithBlockExplorers(polledPayment.address,
			polledPayment.amount, polledPayment.txNotBefore,
			c.cfg.MinConfirmationsRequired)
		if err != nil {
			log.Errorf("cannot fetch tx: %v\n", err)
			continue
		}

		if tx != "" {
			// Update the invoice in the database.
			invoice.Status = v1.InvoiceStatusPaid
			// TOOD: add invoice payments table
			err = c.db.UpdateInvoice(invoice)
			if err != nil {
				if err == database.ErrShutdown {
					// The database is shutdown, so stop the thread.
					return false, nil
				}

				log.Errorf("cannot update invoice with token %v: %v",
					invoice.Token, err)
				continue
			}

			// Remove this invoice from polling.
			tokensToRemove = append(tokensToRemove, token)
			log.Tracef("  removing from polling, invoice just paid")
		}

		time.Sleep(pollCheckGap)
	}

	return true, tokensToRemove
}

func (c *cmswww) removeInvoicesFromPolling(tokensToRemove []string) {
	c.Lock()
	defer c.Unlock()

	for _, token := range tokensToRemove {
		delete(c.polledPayments, token)
	}
}

func (c *cmswww) checkForPayments() {
	for {
		invoicePaymentsToCheck := c.createPolledPaymentsCopy()
		shouldContinue, invoiceTokensToRemove := c.checkForInvoicePayments(invoicePaymentsToCheck)
		if !shouldContinue {
			return
		}
		c.removeInvoicesFromPolling(invoiceTokensToRemove)

		time.Sleep(pollCheckGap)
	}
}

func (c *cmswww) initPaymentChecker() error {
	err := c.addInvoicesForPolling()
	if err != nil {
		return err
	}

	// Start the thread that checks for payments.
	go c.checkForPayments()
	return nil
}
