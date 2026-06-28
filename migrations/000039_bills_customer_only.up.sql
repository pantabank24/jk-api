-- Bills are a customer-only flow. Reconcile databases that received the earlier
-- grants (master with bills.create; owner/employee with bills.read/approve):
--   - master manages bills (issue/approve) but does NOT create them
--   - owner/employee have no bill access at all

-- Revoke bills.create from master
DELETE FROM role_permissions
WHERE role_id IN (SELECT id FROM roles WHERE name = 'master')
  AND permission_id IN (SELECT id FROM permissions WHERE code = 'bills.create');

-- Revoke all bill permissions from owner + employee
DELETE FROM role_permissions
WHERE role_id IN (SELECT id FROM roles WHERE name IN ('owner', 'employee'))
  AND permission_id IN (SELECT id FROM permissions WHERE code IN ('bills.read', 'bills.approve', 'bills.create', 'bills.issue'));
