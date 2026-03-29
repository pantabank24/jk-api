-- Fix service_rate default: 0.0658 gives wrong result; correct value is 0.0656 (≈ 1/15.244 g/baht)
UPDATE gold_types SET service_rate = 0.0656 WHERE name IN (
    'ทองรูปพรรณ',
    'ทองหลอม',
    'กรอบทอง/ตลับทอง',
    'ทอง 9K',
    'ทอง 14K',
    'ทอง 18K',
    'อื่น ๆ'
);
