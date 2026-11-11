-- +goose Up
-- Partner stores sell repair materials (acceptance HT-015): a fourth kind
-- of account, its profile, its products and up to 5 photos per product
-- (the pictures themselves are kept by internal/photos, by key). An admin
-- can hide a product; that decision is logged with the product.
ALTER TABLE accounts DROP CONSTRAINT IF EXISTS accounts_role_check;
ALTER TABLE accounts ADD CONSTRAINT accounts_role_check CHECK (role IN ('customer', 'repairman', 'admin', 'store'));

CREATE TABLE IF NOT EXISTS store_profiles (
    account_id BIGINT PRIMARY KEY REFERENCES accounts (id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    area TEXT NOT NULL,
    address TEXT NOT NULL,
    pickup BOOLEAN NOT NULL,
    delivers BOOLEAN NOT NULL,
    -- Rupiah; 0 when the store does not deliver.
    delivery_fee BIGINT NOT NULL DEFAULT 0 CHECK (delivery_fee >= 0),
    delivery_areas TEXT NOT NULL DEFAULT '',
    updated_at TIMESTAMPTZ NOT NULL,
    CHECK (pickup OR delivers)
);

CREATE TABLE IF NOT EXISTS products (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    store_id BIGINT NOT NULL REFERENCES store_profiles (account_id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    service TEXT NOT NULL,
    -- Rupiah per unit.
    price BIGINT NOT NULL CHECK (price > 0),
    unit TEXT NOT NULL,
    available BOOLEAN NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    -- Set by an admin; the store still sees it.
    hidden BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL
);
CREATE INDEX IF NOT EXISTS products_store ON products (store_id, updated_at DESC);

CREATE TABLE IF NOT EXISTS product_photos (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    product_id BIGINT NOT NULL REFERENCES products (id) ON DELETE CASCADE,
    key TEXT NOT NULL UNIQUE,
    width INT NOT NULL,
    height INT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL
);
CREATE INDEX IF NOT EXISTS product_photos_product ON product_photos (product_id, id);

ALTER TABLE admin_decisions ADD COLUMN IF NOT EXISTS product_id BIGINT REFERENCES products (id) ON DELETE SET NULL;
ALTER TABLE admin_decisions DROP CONSTRAINT IF EXISTS admin_decisions_action_check;
ALTER TABLE admin_decisions ADD CONSTRAINT admin_decisions_action_check
    CHECK (action IN ('points_stand', 'points_removed', 'reopen', 'keep_closed', 'suspend', 'lift', 'hide_product', 'show_product'));

-- +goose Down
-- Fails while store accounts or product decisions exist. Photo files are
-- left in the photos folder.
ALTER TABLE admin_decisions DROP CONSTRAINT IF EXISTS admin_decisions_action_check;
ALTER TABLE admin_decisions ADD CONSTRAINT admin_decisions_action_check
    CHECK (action IN ('points_stand', 'points_removed', 'reopen', 'keep_closed', 'suspend', 'lift'));
ALTER TABLE admin_decisions DROP COLUMN IF EXISTS product_id;
DROP TABLE IF EXISTS product_photos;
DROP TABLE IF EXISTS products;
DROP TABLE IF EXISTS store_profiles;
ALTER TABLE accounts DROP CONSTRAINT IF EXISTS accounts_role_check;
ALTER TABLE accounts ADD CONSTRAINT accounts_role_check CHECK (role IN ('customer', 'repairman', 'admin'));
