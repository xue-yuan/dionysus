from handlers import heartbeat
from utils import APIRouter


apiRouter = APIRouter(prefix="/api")
v1Router = APIRouter(prefix="/v1")
userRouter = APIRouter(prefix="/user", tags=["user"])

userRouter.add_api_route("", heartbeat)
userRouter.add_get("/test_get", heartbeat, auth=True)
userRouter.add_patch("/test_patch", heartbeat, auth=True)
userRouter.add_delete("/test_delete", heartbeat)
userRouter.add_post("/test_post", heartbeat)
userRouter.add_put("/test_put", heartbeat, auth=True)

apiRouter.add_api_route("/heartbeat", heartbeat)

v1Router.include_router(userRouter)
apiRouter.include_router(v1Router)
