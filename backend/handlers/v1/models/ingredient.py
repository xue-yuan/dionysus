from pydantic import BaseModel


from constants.recipe import IngredientType, IngredientUnit


class UpdateRequest(BaseModel):
    name: str


class CreateRequest(UpdateRequest):
    name: str
    unit: IngredientUnit
    type: IngredientType


class Response(CreateRequest):
    id: int = 1
