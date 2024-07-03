from handlers import heartbeat
from utils import APIRouter

from handlers.v1 import cocktail, ingredient
from handlers.v1 import models


userRouter = APIRouter(prefix="/user", tags=["User"])

cocktailRouter = APIRouter(prefix="/cocktail", tags=["Cocktail"])
cocktailRouter.add_get("", cocktail.get_cocktails)

ingredientRouter = APIRouter(prefix="/ingredient", tags=["Ingredient"])
ingredientRouter.add_get("", ingredient.get)
ingredientRouter.add_post("", ingredient.create)
ingredientRouter.add_put("", ingredient.update)
ingredientRouter.add_delete("", ingredient.delete)

v1Router = APIRouter(prefix="/v1")
v1Router.include_router(userRouter)
v1Router.include_router(cocktailRouter)
v1Router.include_router(ingredientRouter)

apiRouter = APIRouter(prefix="/api")
apiRouter.add_get("/heartbeat", heartbeat)
apiRouter.include_router(v1Router)
