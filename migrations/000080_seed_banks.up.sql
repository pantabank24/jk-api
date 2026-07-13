-- Initial bank list. Matched on `code` and skipped when already present, so this
-- can be re-run (or shipped to an existing DB whose staff already added banks)
-- without duplicating rows or clobbering names/sort orders they changed.
INSERT INTO banks (name, code, sort_order, is_active)
SELECT v.name, v.code, v.sort_order, TRUE
FROM (VALUES
    ('ธนาคารกสิกรไทย',                        'KBANK', 1),
    ('ธนาคารไทยพาณิชย์',                      'SCB',   2),
    ('ธนาคารกรุงเทพ',                          'BBL',   3),
    ('ธนาคารกรุงไทย',                          'KTB',   4),
    ('ธนาคารกรุงศรีอยุธยา',                    'BAY',   5),
    ('ธนาคารทหารไทยธนชาต',                    'TTB',   6),
    ('ธนาคารออมสิน',                           'GSB',   7),
    ('ธนาคารเพื่อการเกษตรและสหกรณ์การเกษตร',   'BAAC',  8),
    ('ธนาคารอาคารสงเคราะห์',                   'GHB',   9),
    ('ธนาคารเกียรตินาคินภัทร',                 'KKP',   10),
    ('ธนาคารซีไอเอ็มบี ไทย',                   'CIMBT', 11),
    ('ธนาคารทิสโก้',                           'TISCO', 12),
    ('ธนาคารยูโอบี',                           'UOB',   13),
    ('ธนาคารแลนด์ แอนด์ เฮ้าส์',               'LHBANK',14),
    ('ธนาคารไทยเครดิต',                        'TCRB',  15),
    ('ธนาคารไอซีบีซี (ไทย)',                   'ICBCT', 16),
    ('ธนาคารสแตนดาร์ดชาร์เตอร์ด (ไทย)',        'SCBT',  17),
    ('ธนาคารอิสลามแห่งประเทศไทย',              'IBANK', 18),
    ('ธนาคารเพื่อการส่งออกและนำเข้าแห่งประเทศไทย', 'EXIM', 19),
    ('ธนาคารพัฒนาวิสาหกิจขนาดกลางและขนาดย่อมแห่งประเทศไทย', 'SME', 20)
) AS v(name, code, sort_order)
WHERE NOT EXISTS (SELECT 1 FROM banks b WHERE b.code = v.code);
