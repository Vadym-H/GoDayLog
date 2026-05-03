ALTER TABLE users
  ADD COLUMN plan TEXT NOT NULL DEFAULT 'free'
  CHECK (plan IN ('free', 'pro'));
