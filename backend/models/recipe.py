from sqlalchemy import Column, SMALLINT, Text
from sqlmodel import Field, Relationship, Enum, Session, SQLModel, select

from constants.recipe import IngredientType, IngredientUnit
from database.queries import ingredient


class CocktailIngredient(SQLModel, table=True):
    __tablename__ = "cocktail_ingredients"

    id: int | None = Field(default=None, primary_key=True)
    cocktail_id: int | None = Field(
        default=None, foreign_key="cocktails.id", primary_key=True
    )
    ingredient_id: int | None = Field(
        default=None, foreign_key="ingredients.id", primary_key=True
    )
    quantity: int = Field(sa_column=Column(SMALLINT, nullable=False))

    @classmethod
    def bulk_insert(cls, s: Session, cocktail_id, _ingredients):
        ingredients = []
        for ing in _ingredients:
            ingredients.append(
                cls(
                    cocktail_id=cocktail_id,
                    ingredient_id=_ingredients["id"],
                    quantity=_ingredients["quantity"],
                )
            )

        s.bulk_save_objects(ingredients)
        s.flush()

        return ingredients


class Ingredient(SQLModel, table=True):
    __tablename__ = "ingredients"

    id: int | None = Field(default=None, primary_key=True)
    name: str = Field(nullable=False, unique=True, max_length=63)
    unit: IngredientUnit = Field(sa_column=Column(Enum(IngredientUnit), nullable=False))
    type: IngredientType = Field(sa_column=Column(Enum(IngredientType), nullable=False))

    cocktails: list["Cocktail"] = Relationship(
        back_populates="ingredients", link_model=CocktailIngredient
    )

    @classmethod
    def get_by_id(cls, s: Session, id):
        return s.exec(select(cls).where(cls.id == id)).one()

    @classmethod
    def create(cls, s: Session, name, unit, type):
        ing = cls(
            name=name,
            unit=unit,
            type=type,
        )
        s.add(ing)
        s.flush()

        return ing

    @classmethod
    def update(cls, s: Session, id, name):
        return s.exec(ingredient.update_query(), params={"name": name, "id": id})

    @classmethod
    def delete(cls, s: Session, id):
        return s.exec(ingredient.delete_query(), params={"id": id})


class Cocktail(SQLModel, table=True):
    __tablename__ = "cocktails"

    id: int | None = Field(default=None, primary_key=True)
    name: str = Field(nullable=False, unique=True, max_length=63)
    recipe: str = Field(sa_column=Column(Text, nullable=False))

    ingredients: list["Ingredient"] = Relationship(
        back_populates="cocktails", link_model=CocktailIngredient
    )
    aliases: list["Alias"] = Relationship(back_populates="cocktail")
    tags: list["Tag"] = Relationship(back_populates="cocktail")

    @classmethod
    def get_by_id(cls, s: Session, id):
        return s.exec(select(cls).where(cls.id == id)).one()

    @classmethod
    def create(cls, s: Session, name, ingredients, recipe):
        cocktail = cls(name=name, recipe=recipe)

        s.add(cocktail)
        s.flush()

        CocktailIngredient.bulk_insert(s, cocktail.id, ingredients)

        return cocktail


class Alias(SQLModel, table=True):
    __tablename__ = "aliases"

    id: int | None = Field(default=None, primary_key=True)
    alias: str = Field(nullable=False, unique=True, max_length=63)

    cocktail_id: int = Field(default=None, foreign_key="cocktails.id")
    cocktail: Cocktail | None = Relationship(back_populates="aliases")


class Tag(SQLModel, table=True):
    __tablename__ = "tags"

    id: int | None = Field(default=None, primary_key=True)
    tag: str = Field(nullable=False, unique=True, max_length=31)

    cocktail_id: int = Field(default=None, foreign_key="cocktails.id")
    cocktail: Cocktail | None = Relationship(back_populates="tags")
