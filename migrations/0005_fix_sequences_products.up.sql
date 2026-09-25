SELECT setval(pg_get_serial_sequence('products',   'id'), (SELECT COALESCE(max(id), 1) FROM products));
SELECT setval(pg_get_serial_sequence('placements', 'id'), (SELECT COALESCE(max(id), 1) FROM placements));
SELECT setval(pg_get_serial_sequence('users',      'id'), (SELECT COALESCE(max(id), 1) FROM users));
