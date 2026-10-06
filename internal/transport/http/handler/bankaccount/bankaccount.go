package bankaccount

import (
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/hkizilbulak/haradan-be/internal/application/bankaccount"
	"github.com/hkizilbulak/haradan-be/internal/domain/bank_account"
	"github.com/hkizilbulak/haradan-be/internal/transport/http/generated"
)

type Handler struct {
	service      *bankaccount.Service
	logger       *slog.Logger
	respondError func(c *gin.Context, logger *slog.Logger, err error)
}

func NewHandler(service *bankaccount.Service, logger *slog.Logger, respondError func(c *gin.Context, logger *slog.Logger, err error)) *Handler {
	return &Handler{
		service:      service,
		logger:       logger,
		respondError: respondError,
	}
}

func (h *Handler) GetActiveBankAccounts(c *gin.Context) {
	accounts, err := h.service.GetActiveBankAccounts(c.Request.Context())
	if err != nil {
		h.respondError(c, h.logger, err)
		return
	}

	res := make([]generated.BankAccount, len(accounts))
	for i, a := range accounts {
		res[i] = mapToAPI(a)
	}
	c.JSON(http.StatusOK, res)
}

func (h *Handler) AdminGetBankAccounts(c *gin.Context) {
	accounts, err := h.service.GetAllBankAccounts(c.Request.Context())
	if err != nil {
		h.respondError(c, h.logger, err)
		return
	}

	res := make([]generated.BankAccount, len(accounts))
	for i, a := range accounts {
		res[i] = mapToAPI(a)
	}
	c.JSON(http.StatusOK, res)
}

func (h *Handler) AdminCreateBankAccount(c *gin.Context) {
	var req generated.BankAccountCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	isActive := true
	if req.IsActive != nil {
		isActive = *req.IsActive
	}

	displayOrder := 0
	if req.DisplayOrder != nil {
		displayOrder = *req.DisplayOrder
	}

	p := bank_account.CreateParams{
		BankName:      req.BankName,
		AccountHolder: req.AccountHolder,
		IBAN:          req.Iban,
		BranchName:    req.BranchName,
		AccountNumber: req.AccountNumber,
		IsActive:      isActive,
		DisplayOrder:  displayOrder,
	}

	account, err := h.service.CreateBankAccount(c.Request.Context(), p)
	if err != nil {
		h.respondError(c, h.logger, err)
		return
	}

	c.JSON(http.StatusCreated, mapToAPI(*account))
}

func (h *Handler) AdminUpdateBankAccount(c *gin.Context, id int) {
	var req generated.BankAccountUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	isActive := true
	if req.IsActive != nil {
		isActive = *req.IsActive
	}

	displayOrder := 0
	if req.DisplayOrder != nil {
		displayOrder = *req.DisplayOrder
	}

	p := bank_account.UpdateParams{
		BankName:      req.BankName,
		AccountHolder: req.AccountHolder,
		IBAN:          req.Iban,
		BranchName:    req.BranchName,
		AccountNumber: req.AccountNumber,
		IsActive:      isActive,
		DisplayOrder:  displayOrder,
	}

	account, err := h.service.UpdateBankAccount(c.Request.Context(), id, p)
	if err != nil {
		h.respondError(c, h.logger, err)
		return
	}
	if account == nil {
		c.Status(http.StatusNotFound)
		return
	}

	c.JSON(http.StatusOK, mapToAPI(*account))
}

func (h *Handler) AdminDeleteBankAccount(c *gin.Context, id int) {
	err := h.service.DeleteBankAccount(c.Request.Context(), id)
	if err != nil {
		h.respondError(c, h.logger, err)
		return
	}

	c.Status(http.StatusNoContent)
}

func mapToAPI(a bank_account.BankAccount) generated.BankAccount {
	return generated.BankAccount{
		Id:            a.ID,
		BankName:      a.BankName,
		AccountHolder: a.AccountHolder,
		Iban:          a.IBAN,
		BranchName:    a.BranchName,
		AccountNumber: a.AccountNumber,
		IsActive:      a.IsActive,
		DisplayOrder:  a.DisplayOrder,
		CreatedAt:     a.CreatedAt,
		UpdatedAt:     a.UpdatedAt,
	}
}
