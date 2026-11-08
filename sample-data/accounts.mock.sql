-- Development-only PostgreSQL fixture for sample-data/accounts.mock.json.
-- This file only defines commands; nothing is executed automatically.
-- Run the application migration before this script so the sdms schema,
-- accounts, divisions, sessions, and topic tables exist.

BEGIN;

SET LOCAL search_path = sdms, pg_catalog;
SET LOCAL statement_timeout = '15s';

CREATE TEMP TABLE mock_account_seed (
    uuid uuid PRIMARY KEY,
    username varchar(32) NOT NULL UNIQUE,
    password_hash text NOT NULL,
    prefix varchar(30) NOT NULL,
    first_name varchar(120) NOT NULL,
    last_name varchar(120) NOT NULL,
    is_exec boolean NOT NULL,
    is_admin boolean NOT NULL,
    is_active boolean NOT NULL
) ON COMMIT DROP;

INSERT INTO mock_account_seed (
    uuid,
    username,
    password_hash,
    prefix,
    first_name,
    last_name,
    is_exec,
    is_admin,
    is_active
)
VALUES
    (
        '019a6120-0000-7000-8000-000000000001',
        'admin.mock',
        '$2a$10$3kczlETmnTiz4PEX5zfrKeG523OLYdorDWqK52p5IH8yLI20FtwYC',
        'นาย', 'ธนกร', 'ดูแลระบบ', false, true, true
    ),
    (
        '019a6120-0000-7000-8000-000000000002',
        'executive.mock',
        '$2a$10$h1AjrVPg/rk3SND8ouga2eqjkPTnLq/GE0kwhyD0YpSZ4AGyE0kEi',
        'นาง', 'กมลวรรณ', 'บริหารกิจ', true, false, true
    ),
    (
        '019a6120-0000-7000-8000-000000000003',
        'head.academic',
        '$2a$10$SYWT6xbVdz.SIE56svCPaeZ0EvVI0ILRVMz3/9mD/giCT3V85USDW',
        'นางสาว', 'อรทัย', 'แสงวิชา', false, false, true
    ),
    (
        '019a6120-0000-7000-8000-000000000004',
        'head.budget',
        '$2a$10$isbaaQqNmnjWSbVgv6/UWekd6L1IFse1swU0ctwARpfV1F17GSiNG',
        'นาย', 'วีรพล', 'ทรัพย์มั่น', false, false, true
    ),
    (
        '019a6120-0000-7000-8000-000000000005',
        'head.personnel',
        '$2a$10$Dxik0NOl2PYkIXlUGpA2OeSQa2HnFjvzltpYQA0qBRFD4WEp0zTKe',
        'นาง', 'สุมาลี', 'คนดี', false, false, true
    ),
    (
        '019a6120-0000-7000-8000-000000000006',
        'head.general',
        '$2a$10$TG0sSeAgZHZ7rZlAlNBHaeNd2m8CKt3jg1qSieC2ajRDbs0Qnny6K',
        'นาย', 'ชาญชัย', 'งานพร้อม', false, false, true
    ),
    (
        '019a6120-0000-7000-8000-000000000007',
        'teacher.mock01',
        '$2a$10$ztqIxMRxxKl8rRp4f5WM8eY3hY3GJnJ33DGcgRsH0z0aAlGF/eNbW',
        'นาย', 'กิตติพงษ์', 'รุ่งเรือง', false, false, true
    ),
    (
        '019a6120-0000-7000-8000-000000000008',
        'teacher.mock02',
        '$2a$10$y1P55GSwKjFBKAmtZu7aKeT/p5kdTB3VDe2t9wV3FGdzoe3VpPHLS',
        'นางสาว', 'นภัสสร', 'เงินดี', false, false, true
    ),
    (
        '019a6120-0000-7000-8000-000000000009',
        'officer.personnel01',
        '$2a$10$.8efRAZnfG1W1wjW2BwubOE7UejzNKMqZzHCP1njEUmzwt.Cb2I6e',
        'นางสาว', 'วราภรณ์', 'บุญช่วย', false, false, true
    ),
    (
        '019a6120-0000-7000-8000-00000000000a',
        'officer.general01',
        '$2a$10$LNKi6TCkUmTrCPihL2TCqu8YXUeOLfT0Dv/F0sWMX/gapXFdW22We',
        'นาย', 'อดิศร', 'พร้อมงาน', false, false, true
    ),
    (
        '019a6120-0000-7000-8000-00000000000b',
        'teacher.inactive',
        '$2a$10$MkfjmFLiWyV9vpLPkr9XP.2CKravQYBGGXwsQE193/59acGZ/COXm',
        'นางสาว', 'พิชญา', 'ทดสอบปิดบัญชี', false, false, false
    );

DO $block$
DECLARE
    missing_codes text;
BEGIN
    SELECT string_agg(expected.code, ', ' ORDER BY expected.code)
    INTO missing_codes
    FROM (VALUES ('aca'), ('bud'), ('per'), ('gen')) AS expected(code)
    WHERE NOT EXISTS (
        SELECT 1
        FROM divisions AS division
        WHERE division.code = expected.code
    );

    IF missing_codes IS NOT NULL THEN
        RAISE EXCEPTION 'missing required division codes: %; run the application migration first', missing_codes;
    END IF;
END
$block$;

-- Existing mock accounts keep their current password hash. Profile, role, and
-- activation fields are synchronized with the fixture on every run.
INSERT INTO accounts AS account (
    uuid,
    username,
    password_hash,
    prefix,
    first_name,
    last_name,
    is_exec,
    is_admin,
    deactivated_at,
    created_at,
    updated_at
)
SELECT
    seed.uuid,
    seed.username,
    seed.password_hash,
    seed.prefix,
    seed.first_name,
    seed.last_name,
    seed.is_exec,
    seed.is_admin,
    CASE
        WHEN seed.is_active THEN NULL
        ELSE timestamptz '2026-11-08 09:00:00+07'
    END,
    CURRENT_TIMESTAMP,
    CURRENT_TIMESTAMP
FROM mock_account_seed AS seed
ON CONFLICT (username) DO UPDATE
SET prefix = EXCLUDED.prefix,
    first_name = EXCLUDED.first_name,
    last_name = EXCLUDED.last_name,
    is_exec = EXCLUDED.is_exec,
    is_admin = EXCLUDED.is_admin,
    deactivated_at = EXCLUDED.deactivated_at,
    updated_at = CURRENT_TIMESTAMP;

-- Head-of-division is stored on divisions, not accounts.
UPDATE divisions AS division
SET head_user_uuid = account.uuid
FROM (
    VALUES
        ('aca', 'head.academic'),
        ('bud', 'head.budget'),
        ('per', 'head.personnel'),
        ('gen', 'head.general')
) AS configured(code, username)
JOIN accounts AS account
  ON account.username = configured.username
WHERE division.code = configured.code;

-- Keep disabled-account behavior aligned with the application service.
DELETE FROM sessions AS session
USING accounts AS account, mock_account_seed AS seed
WHERE seed.is_active = false
  AND account.username = seed.username
  AND session.user_uuid = account.uuid;

DELETE FROM topic_assignees AS assignment
USING topics AS topic, accounts AS account, mock_account_seed AS seed
WHERE seed.is_active = false
  AND account.username = seed.username
  AND assignment.user_uuid = account.uuid
  AND assignment.topic_uuid = topic.uuid
  AND topic.is_open = true
  AND NOT EXISTS (
      SELECT 1
      FROM submissions AS submission
      WHERE submission.topic_uuid = assignment.topic_uuid
        AND submission.user_uuid = assignment.user_uuid
  );

-- Preview the synchronized mock accounts before the transaction commits.
SELECT
    account.username,
    account.prefix,
    account.first_name,
    account.last_name,
    account.is_exec,
    account.is_admin,
    account.deactivated_at IS NULL AS is_active,
    ARRAY(
        SELECT division.code
        FROM divisions AS division
        ORDER BY division.sort_order, division.code
    ) AS division_codes
FROM accounts AS account
JOIN mock_account_seed AS seed
  ON seed.username = account.username
ORDER BY account.username;

COMMIT;
