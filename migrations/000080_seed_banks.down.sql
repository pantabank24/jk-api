-- Only remove seeded banks nobody is using; a bank a customer was assigned to is
-- real data now and must not be dropped by rolling back the seed.
DELETE FROM banks
WHERE code IN ('KBANK','SCB','BBL','KTB','BAY','TTB','GSB','BAAC','GHB','KKP','CIMBT',
               'TISCO','UOB','LHBANK','TCRB','ICBCT','SCBT','IBANK','EXIM','SME')
  AND NOT EXISTS (SELECT 1 FROM users u WHERE u.bank_id = banks.id);
