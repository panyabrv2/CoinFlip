INSERT INTO twist_business.users (user_id, username, registration_date)
VALUES
    (1, 'test1', now()),
    (2, 'test2', now())
    ON CONFLICT (user_id) DO NOTHING;

INSERT INTO twist_business.items (
    item_id, name, photo_url, cost_ton, user_id, locked, type, can_withdraw
)
VALUES
    (1001, 'Gift A', NULL, 5.0000, 1, false, 'gift', true),
    (1002, 'Gift B', NULL, 7.5000, 1, false, 'gift', true),
    (1003, 'Card C', NULL, 3.0000, 1, false, 'card', true),

    (2001, 'Gift X', NULL, 4.2000, 2, false, 'gift', true),
    (2002, 'Card Y', NULL, 9.9000, 2, false, 'card', true)
    ON CONFLICT (item_id) DO NOTHING;

