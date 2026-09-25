INSERT INTO floors (id, code, name) VALUES
  (1, '1', 'Первый этаж'),
  (2, '2', 'Второй этаж');

INSERT INTO racks (id, floor_id, code) VALUES
  (1, 1, 'A'),
  (2, 1, 'B'),
  (3, 2, 'A'),
  (4, 2, 'C');

INSERT INTO shelves (id, rack_id, code) VALUES
  (1, 1, '01'), (2, 1, '02'),
  (3, 2, '01'),
  (4, 3, '01'), (5, 3, '02'),
  (6, 4, '01');

INSERT INTO cells (id, shelf_id, code) VALUES
  (1, 1, '01'), (2, 1, '02'), (3, 1, '03'),
  (4, 2, '01'), (5, 2, '02'),
  (6, 3, '01'),
  (7, 4, '01'), (8, 4, '02'),
  (9, 5, '01'),
  (10, 6, '01'), (11, 6, '02'), (12, 6, '03');