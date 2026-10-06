import re

with open('cmd/api/main.go', 'r') as f:
    content = f.read()

# Add imports
import_repo = '"github.com/hkizilbulak/haradan-be/internal/infrastructure/postgres/bank_account"'
import_svc = '"github.com/hkizilbulak/haradan-be/internal/application/bankaccount"'
if import_repo not in content:
    content = re.sub(r'("github.com/hkizilbulak/haradan-be/internal/infrastructure/postgres/studfarm")', r'\1\n\tpbankaccount "github.com/hkizilbulak/haradan-be/internal/infrastructure/postgres/bank_account"', content)
if import_svc not in content:
    content = re.sub(r'("github.com/hkizilbulak/haradan-be/internal/application/studfarm")', r'\1\n\tappbankaccount "github.com/hkizilbulak/haradan-be/internal/application/bankaccount"', content)


# Add instantiation
instantiation = """
	bankAccountRepo := pbankaccount.NewRepository(pool)
	bankAccountSvc := appbankaccount.NewService(bankAccountRepo)
"""
if 'bankAccountRepo :=' not in content:
    content = re.sub(r'(studFarmSvc := appstudfarm.NewService\(studFarmRepo, aiSvc, appGeo\))', r'\1\n' + instantiation, content)

# Add WithBankAccountService to server
if 'WithBankAccountService' not in content:
    content = re.sub(r'(\.WithStudFarmService\(studFarmSvc\))', r'\1\n\t\t.WithBankAccountService(bankAccountSvc)', content)

with open('cmd/api/main.go', 'w') as f:
    f.write(content)
