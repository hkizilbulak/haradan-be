import re

with open('internal/transport/http/handler/bankaccount/bankaccount.go', 'r') as f:
    content = f.read()

# Fix parsing of optional values
content = content.replace('IsActive:      req.IsActive,', 'IsActive:      req.IsActive != None && *req.IsActive,')
content = content.replace('DisplayOrder:  req.DisplayOrder,', 'DisplayOrder:  *req.DisplayOrder,') # Let's handle safely below

def safe_deref(req_prop, type_val, default):
    return f"""	{req_prop}_val := {default}
	if req.{req_prop} != nil {{
		{req_prop}_val = *req.{req_prop}
	}}"""

content = re.sub(
    r'(p := bank_account\.CreateParams{)',
    r'''	isActive_val := true
	if req.IsActive != nil {
		isActive_val = *req.IsActive
	}
	displayOrder_val := 0
	if req.DisplayOrder != nil {
		displayOrder_val = *req.DisplayOrder
	}
	\1''', content, count=1)
content = content.replace('IsActive:      req.IsActive,', 'IsActive:      isActive_val,', 1)
content = content.replace('DisplayOrder:  req.DisplayOrder,', 'DisplayOrder:  displayOrder_val,', 1)

content = re.sub(
    r'(p := bank_account\.UpdateParams{)',
    r'''	isActive_val := true
	if req.IsActive != nil {
		isActive_val = *req.IsActive
	}
	displayOrder_val := 0
	if req.DisplayOrder != nil {
		displayOrder_val = *req.DisplayOrder
	}
	\1''', content, count=1)
content = content.replace('IsActive:      req.IsActive,', 'IsActive:      isActive_val,', 1)
content = content.replace('DisplayOrder:  req.DisplayOrder,', 'DisplayOrder:  displayOrder_val,', 1)


# Fix mapToAPI
new_map = """func mapToAPI(a bank_account.BankAccount) generated.BankAccount {
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
}"""
content = re.sub(r'func mapToAPI.*?^}', new_map, content, flags=re.MULTILINE|re.DOTALL)

with open('internal/transport/http/handler/bankaccount/bankaccount.go', 'w') as f:
    f.write(content)
