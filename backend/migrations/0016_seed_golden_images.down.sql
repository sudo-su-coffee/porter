-- Rollback 0016: remove the seeded golden images (leaves user uploads intact).
DELETE FROM golden_images WHERE name IN ('redis', 'postgresql', 'mysql');
