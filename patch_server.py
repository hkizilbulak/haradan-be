import re

with open('internal/transport/http/handler/server.go', 'r') as f:
    content = f.read()

# Add import
import_str = '"github.com/hkizilbulak/haradan-be/internal/transport/http/handler/bankaccount"'
if import_str not in content:
    content = re.sub(r'("github.com/hkizilbulak/haradan-be/internal/transport/http/handler/auth")', r'\1\n\tbankaccounthandler "github.com/hkizilbulak/haradan-be/internal/transport/http/handler/bankaccount"', content)

# Add application import
app_import_str = '"github.com/hkizilbulak/haradan-be/internal/application/bankaccount"'
if app_import_str not in content:
    content = re.sub(r'("github.com/hkizilbulak/haradan-be/internal/application/auth")', r'\1\n\tappbankaccount "github.com/hkizilbulak/haradan-be/internal/application/bankaccount"', content)

# Add to Server struct
if 'bankaccount *bankaccounthandler.Handler' not in content:
    content = re.sub(r'(communicationTemplate \*communicationtemplatehandler\.Handler)', r'\1\n\tbankaccount *bankaccounthandler.Handler', content)

# Add WithBankAccountService
if 'WithBankAccountService' not in content:
    with_func = """
func (s *Server) WithBankAccountService(svc *appbankaccount.Service) *Server {
	if svc != nil {
		s.bankaccount = bankaccounthandler.NewHandler(svc, s.logger, respondError)
	}
	return s
}
"""
    content = content + with_func

# Add API method delegations
if 'GetActiveBankAccounts' not in content:
    delegations = """
func (s *Server) GetActiveBankAccounts(c *gin.Context) {
	if s.bankaccount != nil {
		s.bankaccount.GetActiveBankAccounts(c)
	} else {
		respondNotImplemented(c)
	}
}

func (s *Server) AdminGetBankAccounts(c *gin.Context) {
	if s.bankaccount != nil {
		s.bankaccount.AdminGetBankAccounts(c)
	} else {
		respondNotImplemented(c)
	}
}

func (s *Server) AdminCreateBankAccount(c *gin.Context) {
	if s.bankaccount != nil {
		s.bankaccount.AdminCreateBankAccount(c)
	} else {
		respondNotImplemented(c)
	}
}

func (s *Server) AdminUpdateBankAccount(c *gin.Context, id int) {
	if s.bankaccount != nil {
		s.bankaccount.AdminUpdateBankAccount(c, id)
	} else {
		respondNotImplemented(c)
	}
}

func (s *Server) AdminDeleteBankAccount(c *gin.Context, id int) {
	if s.bankaccount != nil {
		s.bankaccount.AdminDeleteBankAccount(c, id)
	} else {
		respondNotImplemented(c)
	}
}
"""
    content = content + delegations

with open('internal/transport/http/handler/server.go', 'w') as f:
    f.write(content)
