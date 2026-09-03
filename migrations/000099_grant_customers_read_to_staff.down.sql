-- คืนสภาพเป็นแบบ migration 000036: customers.read เป็นของ master คนเดียว.
DELETE FROM role_permissions
WHERE role_id IN (SELECT id FROM roles WHERE name IN ('owner', 'employee'))
  AND permission_id IN (SELECT id FROM permissions WHERE code = 'customers.read');
