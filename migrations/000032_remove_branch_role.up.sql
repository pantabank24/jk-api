-- Reduce roles to master / owner / employee by removing the 'branch' (ผู้จัดการสาขา) role.
-- For databases already seeded with the branch role, reassign any branch users to
-- employee first (users.role_id is ON DELETE SET NULL, so deleting the role would
-- otherwise orphan them). Branch users already have a branch_id, which employee also
-- requires, so the reassignment leaves no inconsistent data.
UPDATE users u
SET role_id = e.id,
    role = 'employee'
FROM roles b, roles e
WHERE u.role_id = b.id
  AND b.name = 'branch'
  AND e.name = 'employee';

-- Delete the branch role; role_permissions rows cascade automatically.
DELETE FROM roles WHERE name = 'branch';
