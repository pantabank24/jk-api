DELETE FROM gold_types WHERE name IN (
    'ทองคำแท่ง 96.5%','ทองรูปพรรณ','ทองหลอม','กรอบทอง/ตลับทอง',
    'ทอง 9K','ทอง 14K','ทอง 18K','อื่น ๆ'
);
ALTER TABLE gold_types DROP COLUMN IF EXISTS service_rate;
ALTER TABLE gold_types DROP COLUMN IF EXISTS plus_type;
