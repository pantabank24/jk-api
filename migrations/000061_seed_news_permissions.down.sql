DELETE FROM role_permissions WHERE permission_id IN (SELECT id FROM permissions WHERE code IN ('news.read', 'news.create', 'news.update', 'news.delete'));
DELETE FROM permissions WHERE code IN ('news.read', 'news.create', 'news.update', 'news.delete');
