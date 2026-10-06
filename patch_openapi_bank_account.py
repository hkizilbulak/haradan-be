import yaml

with open('api/openapi.yaml', 'r') as f:
    spec = yaml.safe_load(f)

# Add schema
spec['components']['schemas']['BankAccount'] = {
    'type': 'object',
    'properties': {
        'id': {'type': 'integer'},
        'bank_name': {'type': 'string'},
        'account_holder': {'type': 'string'},
        'iban': {'type': 'string'},
        'branch_name': {'type': 'string', 'nullable': True},
        'account_number': {'type': 'string', 'nullable': True},
        'is_active': {'type': 'boolean'},
        'display_order': {'type': 'integer'},
        'created_at': {'type': 'string', 'format': 'date-time'},
        'updated_at': {'type': 'string', 'format': 'date-time'}
    },
    'required': ['id', 'bank_name', 'account_holder', 'iban', 'is_active', 'display_order', 'created_at', 'updated_at']
}

spec['components']['schemas']['BankAccountCreateRequest'] = {
    'type': 'object',
    'properties': {
        'bank_name': {'type': 'string'},
        'account_holder': {'type': 'string'},
        'iban': {'type': 'string'},
        'branch_name': {'type': 'string', 'nullable': True},
        'account_number': {'type': 'string', 'nullable': True},
        'is_active': {'type': 'boolean', 'default': True},
        'display_order': {'type': 'integer', 'default': 0}
    },
    'required': ['bank_name', 'account_holder', 'iban']
}

spec['components']['schemas']['BankAccountUpdateRequest'] = spec['components']['schemas']['BankAccountCreateRequest']

# Add endpoints
spec['paths']['/v1/bank-accounts/active'] = {
    'get': {
        'summary': 'Get active bank accounts',
        'operationId': 'GetActiveBankAccounts',
        'tags': ['BankAccounts'],
        'responses': {
            '200': {
                'description': 'OK',
                'content': {
                    'application/json': {
                        'schema': {
                            'type': 'array',
                            'items': {'$ref': '#/components/schemas/BankAccount'}
                        }
                    }
                }
            }
        }
    }
}

spec['paths']['/v1/admin/bank-accounts'] = {
    'get': {
        'summary': 'Get all bank accounts (Admin)',
        'operationId': 'AdminGetBankAccounts',
        'tags': ['AdminBankAccounts'],
        'security': [{'AdminSessionAuth': []}],
        'responses': {
            '200': {
                'description': 'OK',
                'content': {
                    'application/json': {
                        'schema': {
                            'type': 'array',
                            'items': {'$ref': '#/components/schemas/BankAccount'}
                        }
                    }
                }
            }
        }
    },
    'post': {
        'summary': 'Create bank account',
        'operationId': 'AdminCreateBankAccount',
        'tags': ['AdminBankAccounts'],
        'security': [{'AdminSessionAuth': []}],
        'requestBody': {
            'required': True,
            'content': {
                'application/json': {
                    'schema': {'$ref': '#/components/schemas/BankAccountCreateRequest'}
                }
            }
        },
        'responses': {
            '201': {
                'description': 'Created',
                'content': {
                    'application/json': {
                        'schema': {'$ref': '#/components/schemas/BankAccount'}
                    }
                }
            }
        }
    }
}

spec['paths']['/v1/admin/bank-accounts/{id}'] = {
    'put': {
        'summary': 'Update bank account',
        'operationId': 'AdminUpdateBankAccount',
        'tags': ['AdminBankAccounts'],
        'security': [{'AdminSessionAuth': []}],
        'parameters': [
            {
                'name': 'id',
                'in': 'path',
                'required': True,
                'schema': {'type': 'integer'}
            }
        ],
        'requestBody': {
            'required': True,
            'content': {
                'application/json': {
                    'schema': {'$ref': '#/components/schemas/BankAccountUpdateRequest'}
                }
            }
        },
        'responses': {
            '200': {
                'description': 'OK',
                'content': {
                    'application/json': {
                        'schema': {'$ref': '#/components/schemas/BankAccount'}
                    }
                }
            }
        }
    },
    'delete': {
        'summary': 'Delete bank account',
        'operationId': 'AdminDeleteBankAccount',
        'tags': ['AdminBankAccounts'],
        'security': [{'AdminSessionAuth': []}],
        'parameters': [
            {
                'name': 'id',
                'in': 'path',
                'required': True,
                'schema': {'type': 'integer'}
            }
        ],
        'responses': {
            '204': {
                'description': 'No Content'
            }
        }
    }
}

with open('api/openapi.yaml', 'w') as f:
    yaml.dump(spec, f, sort_keys=False)
