-- Dionysus schema. Applied on every API start, so every statement must be idempotent.
-- gen_random_uuid() is built into PostgreSQL 13+, so no extension is required.

CREATE TABLE IF NOT EXISTS ingredients (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name TEXT UNIQUE NOT NULL,
    category TEXT NOT NULL,
    translation_key TEXT NOT NULL DEFAULT '',
    -- Interchangeable bottles share a family (e.g. every whiskey is 'Whiskey').
    -- Empty means the ingredient can never be substituted.
    family TEXT NOT NULL DEFAULT ''
);

CREATE TABLE IF NOT EXISTS recipes (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    title TEXT UNIQUE NOT NULL,
    description TEXT,
    glassware TEXT,
    method TEXT NOT NULL,
    steps TEXT NOT NULL,
    image_url TEXT DEFAULT '',
    sweetness INT DEFAULT 3,
    sourness INT DEFAULT 3,
    strength INT DEFAULT 3,
    translation_key TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS recipe_ingredients (
    recipe_id UUID REFERENCES recipes(id) ON DELETE CASCADE,
    ingredient_id UUID REFERENCES ingredients(id) ON DELETE CASCADE,
    amount TEXT NOT NULL,
    unit TEXT NOT NULL,
    -- TRUE when this exact bottle is what makes the drink (the Islay float in a
    -- Penicillin). Family substitution is never offered for strict requirements.
    strict BOOLEAN NOT NULL DEFAULT FALSE,
    PRIMARY KEY (recipe_id, ingredient_id)
);

CREATE TABLE IF NOT EXISTS tags (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name TEXT UNIQUE NOT NULL,
    type TEXT NOT NULL DEFAULT 'Palate', -- 'Palate' or 'Equipment'
    translation_key TEXT NOT NULL DEFAULT ''
);

CREATE TABLE IF NOT EXISTS recipe_tags (
    recipe_id UUID REFERENCES recipes(id) ON DELETE CASCADE,
    tag_id UUID REFERENCES tags(id) ON DELETE CASCADE,
    PRIMARY KEY (recipe_id, tag_id)
);

-- Columns added after the initial release; these keep existing databases current.
ALTER TABLE ingredients        ADD COLUMN IF NOT EXISTS family TEXT NOT NULL DEFAULT '';
ALTER TABLE tags               ADD COLUMN IF NOT EXISTS translation_key TEXT NOT NULL DEFAULT '';
ALTER TABLE recipe_ingredients ADD COLUMN IF NOT EXISTS strict BOOLEAN NOT NULL DEFAULT FALSE;

-- The composite primary keys lead with recipe_id, so lookups starting from an
-- ingredient or a tag have no usable index without these.
CREATE INDEX IF NOT EXISTS idx_recipe_ingredients_ingredient ON recipe_ingredients(ingredient_id);
CREATE INDEX IF NOT EXISTS idx_recipe_tags_tag               ON recipe_tags(tag_id);
CREATE INDEX IF NOT EXISTS idx_ingredients_family            ON ingredients(family) WHERE family <> '';
-- Gallery ordering: strength is the sort key, id is the tiebreaker that keeps
-- pagination stable across requests.
CREATE INDEX IF NOT EXISTS idx_recipes_strength_id           ON recipes(strength, id);
