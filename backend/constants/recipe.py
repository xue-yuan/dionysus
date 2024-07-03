from enum import Enum


class IngredientType(Enum):
    BASE = 0
    LIQUEUR = 1
    WINE = 2
    BEER = 3
    BITTER = 4
    SYRUP = 5
    FRUIT = 6
    MIXER = 7


class IngredientUnit(Enum):
    ML = 0
    DROP = 1
