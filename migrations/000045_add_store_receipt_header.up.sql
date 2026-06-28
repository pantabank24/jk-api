-- Receipt-header fields for stores (shown on the quotation document). Name,
-- address, phone and logo already exist; add the tax id and website.
ALTER TABLE stores ADD COLUMN IF NOT EXISTS tax_id  VARCHAR(50)  DEFAULT '';
ALTER TABLE stores ADD COLUMN IF NOT EXISTS website VARCHAR(255) DEFAULT '';
