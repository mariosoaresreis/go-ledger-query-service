package api

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"go-ledger-query-service/internal/repository"
	"go-ledger-query-service/internal/services"
)

// AccountQueryHandler serves all read-side account endpoints.
type AccountQueryHandler struct {
	svc services.QueryService
}

// NewAccountQueryHandler creates an AccountQueryHandler.
func NewAccountQueryHandler(svc services.QueryService) *AccountQueryHandler {
	return &AccountQueryHandler{svc: svc}
}

// GetBalance godoc
//
//	@Summary		Get account balance
//	@Description	Returns the current balance and status for an account
//	@Tags			accounts
//	@Produce		json
//	@Param			accountId	path		string	true	"Account ID"
//	@Success		200			{object}	domain.AccountBalance
//	@Failure		404			{object}	ErrorResponse
//	@Failure		500			{object}	ErrorResponse
//	@Router			/v1/accounts/{accountId}/balance [get]
func (h *AccountQueryHandler) GetBalance(c *gin.Context) {
	accountID := c.Param("accountId")
	balance, err := h.svc.GetBalance(c.Request.Context(), accountID)
	if err != nil {
		writeError(c.Writer, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(c.Writer, http.StatusOK, balance)
}

// ListTransactions godoc
//
//	@Summary		List transaction history
//	@Description	Returns paginated transaction history for an account
//	@Tags			accounts
//	@Produce		json
//	@Param			accountId	path		string	true	"Account ID"
//	@Param			page		query		int		false	"Page number (0-based)"	default(0)
//	@Param			size		query		int		false	"Page size"				default(20)
//	@Param			from		query		string	false	"From date (YYYY-MM-DD)"
//	@Param			to			query		string	false	"To date (YYYY-MM-DD)"
//	@Param			direction	query		string	false	"CREDIT or DEBIT"		Enums(CREDIT, DEBIT)
//	@Success		200			{object}	PaginatedResponse
//	@Failure		500			{object}	ErrorResponse
//	@Router			/v1/accounts/{accountId}/transactions [get]
func (h *AccountQueryHandler) ListTransactions(c *gin.Context) {
	accountID := c.Param("accountId")
	page, _ := strconv.Atoi(c.DefaultQuery("page", "0"))
	size, _ := strconv.Atoi(c.DefaultQuery("size", "20"))

	filter := repository.TransactionFilter{
		AccountID: accountID,
		From:      c.Query("from"),
		To:        c.Query("to"),
		Direction: c.Query("direction"),
		Page:      page,
		Size:      size,
	}

	txns, total, err := h.svc.ListTransactions(c.Request.Context(), filter)
	if err != nil {
		writeError(c.Writer, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(c.Writer, http.StatusOK, PaginatedResponse{
		Data:       txns,
		Page:       page,
		Size:       size,
		TotalCount: total,
	})
}

// GetStatement godoc
//
//	@Summary		Get monthly statement
//	@Description	Returns the opening balance, all entries, and closing balance for a given month
//	@Tags			accounts
//	@Produce		json
//	@Param			accountId	path		string	true	"Account ID"
//	@Param			month		query		string	true	"Month (YYYY-MM)"
//	@Success		200			{object}	domain.Statement
//	@Failure		400			{object}	ErrorResponse
//	@Failure		500			{object}	ErrorResponse
//	@Router			/v1/accounts/{accountId}/statement [get]
func (h *AccountQueryHandler) GetStatement(c *gin.Context) {
	accountID := c.Param("accountId")
	month := c.Query("month")
	if month == "" {
		writeError(c.Writer, http.StatusBadRequest, "month query parameter is required (YYYY-MM)")
		return
	}

	stmt, err := h.svc.GetStatement(c.Request.Context(), accountID, month)
	if err != nil {
		writeError(c.Writer, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(c.Writer, http.StatusOK, stmt)
}

// ListAccountsByOwner godoc
//
//	@Summary		List accounts by owner
//	@Description	Returns all accounts belonging to an owner
//	@Tags			accounts
//	@Produce		json
//	@Param			ownerId	query		string	true	"Owner ID"
//	@Success		200		{array}		domain.AccountSummary
//	@Failure		400		{object}	ErrorResponse
//	@Failure		500		{object}	ErrorResponse
//	@Router			/v1/accounts [get]
func (h *AccountQueryHandler) ListAccountsByOwner(c *gin.Context) {
	ownerID := c.Query("ownerId")
	if ownerID == "" {
		writeError(c.Writer, http.StatusBadRequest, "ownerId query parameter is required")
		return
	}

	accounts, err := h.svc.ListAccountsByOwner(c.Request.Context(), ownerID)
	if err != nil {
		writeError(c.Writer, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(c.Writer, http.StatusOK, accounts)
}
