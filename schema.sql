-- Enable UUID extension
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- Drop tables for full reset
-- DROP TABLE IF EXISTS recipe_tags CASCADE;
-- DROP TABLE IF EXISTS recipe_ingredients CASCADE;
-- DROP TABLE IF EXISTS tags CASCADE;
-- DROP TABLE IF EXISTS ingredients CASCADE;
-- DROP TABLE IF EXISTS recipes CASCADE;

CREATE TABLE ingredients (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name TEXT UNIQUE NOT NULL,
    category TEXT NOT NULL
);

CREATE TABLE recipes (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    title TEXT UNIQUE NOT NULL,
    description TEXT,
    glassware TEXT,
    method TEXT NOT NULL,
    steps TEXT NOT NULL,
    image_url TEXT DEFAULT '',
    sweetness INT DEFAULT 3,
    sourness INT DEFAULT 3,
    strength INT DEFAULT 3,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE recipe_ingredients (
    recipe_id UUID REFERENCES recipes(id) ON DELETE CASCADE,
    ingredient_id UUID REFERENCES ingredients(id) ON DELETE CASCADE,
    amount TEXT NOT NULL,
    unit TEXT NOT NULL,
    PRIMARY KEY (recipe_id, ingredient_id)
);

CREATE TABLE tags (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name TEXT UNIQUE NOT NULL,
    type TEXT NOT NULL DEFAULT 'Palate' -- 'Palate' or 'Equipment'
);

CREATE TABLE recipe_tags (
    recipe_id UUID REFERENCES recipes(id) ON DELETE CASCADE,
    tag_id UUID REFERENCES tags(id) ON DELETE CASCADE,
    PRIMARY KEY (recipe_id, tag_id)
);

