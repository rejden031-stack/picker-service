SELECT setval(pg_get_serial_sequence('floors',  'id'), (SELECT COALESCE(max(id), 1) FROM floors));
SELECT setval(pg_get_serial_sequence('racks',   'id'), (SELECT COALESCE(max(id), 1) FROM racks));
SELECT setval(pg_get_serial_sequence('shelves', 'id'), (SELECT COALESCE(max(id), 1) FROM shelves));
SELECT setval(pg_get_serial_sequence('cells',   'id'), (SELECT COALESCE(max(id), 1) FROM cells));
