CREATE TABLE floors (
    id   BIGSERIAL PRIMARY KEY,
    code TEXT UNIQUE NOT NULL,
    name TEXT NOT NULL DEFAULT ''
);

CREATE TABLE racks (
    id       BIGSERIAL PRIMARY KEY,
    floor_id BIGINT NOT NULL REFERENCES floors(id),
    code     TEXT NOT NULL,
    UNIQUE (floor_id, code)
);

CREATE TABLE shelves (
    id      BIGSERIAL PRIMARY KEY,
    rack_id BIGINT NOT NULL REFERENCES racks(id),
    code    TEXT NOT NULL,
    UNIQUE (rack_id, code)
);

CREATE TABLE cells (
    id       BIGSERIAL PRIMARY KEY,
    shelf_id BIGINT NOT NULL REFERENCES shelves(id),
    code     TEXT NOT NULL,
    UNIQUE (shelf_id, code)
);

CREATE TABLE products (
    id      BIGSERIAL PRIMARY KEY,
    sku     TEXT UNIQUE NOT NULL,
    name    TEXT NOT NULL,
    barcode TEXT
);

CREATE TABLE placements (
    id         BIGSERIAL PRIMARY KEY,
    product_id BIGINT NOT NULL REFERENCES products(id),
    cell_id    BIGINT NOT NULL REFERENCES cells(id),
    qty        INT NOT NULL DEFAULT 0,
    is_primary BOOLEAN NOT NULL DEFAULT false,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (product_id, cell_id)
);

CREATE INDEX idx_placements_cell    ON placements(cell_id);
CREATE INDEX idx_placements_product ON placements(product_id);