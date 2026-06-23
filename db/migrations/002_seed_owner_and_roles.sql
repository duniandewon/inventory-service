-- Seed the MVP role catalogue and the initial owner account.
-- Accounts are never auto-created at login; the owner must exist before first login.

-- Normalize a lowercase "owner" role from earlier ad-hoc manual seeding to
-- the documented naming (context/features/authentication.md uses OWNER/ADMIN).
UPDATE roles SET name = 'OWNER' WHERE name = 'owner';

INSERT INTO roles (name, description) VALUES
    ('OWNER', 'Full access to all features.'),
    ('OPERATOR', 'Warehouse goods movements.')
ON CONFLICT (name) DO NOTHING;

-- Only seed a placeholder owner on a fresh database with no users yet.
-- Placeholder contact info: WhatsApp sending is stubbed (LoggingOTPSender) for
-- now, so these values are not yet used to deliver real OTP codes. Update
-- before going live.
INSERT INTO users (email, phone_number, full_name)
SELECT 'owner@example.com', '+6281190090680', 'Factory Owner'
WHERE NOT EXISTS (SELECT 1 FROM users);

INSERT INTO user_roles (user_id, role_id)
SELECT u.id, r.id
FROM users u
JOIN roles r ON r.name = 'OWNER'
WHERE u.id = (SELECT MIN(id) FROM users)
ON CONFLICT DO NOTHING;
