ALTER TABLE couriers ADD COLUMN IF NOT EXISTS approved BOOLEAN NOT NULL DEFAULT FALSE;

-- Demo kuryerlar avvaldan ishlayotgan edi — tasdiqlangan deb belgilaymiz
UPDATE couriers SET approved = TRUE WHERE id IN ('c1', 'c2', 'c3');
