from uuid import uuid5

from fastapi import APIRouter as FastAPIRouter, Depends
from fastapi.openapi.utils import get_openapi as _get_openapi

from utils import http
from utils.security import jwt_scheme


class APIRouter(FastAPIRouter):

    def add_api_auth_route(self, path, endpoint, *, methods=None, **kwargs):
        if methods is None:
            methods = ["GET"]

        dependencies = kwargs.get("dependencies", [])
        dependencies.append(Depends(jwt_scheme))
        kwargs["dependencies"] = dependencies

        self.add_api_route(path, endpoint, methods=methods, **kwargs)

    def _add_api_route(self, path, endpoint, auth=False, *, methods, **kwargs):
        if auth:
            self.add_api_auth_route(path, endpoint, methods=methods, **kwargs)
            return

        self.add_api_route(path, endpoint, methods=methods, **kwargs)

    def add_get(self, path, endpoint, auth=False, **kwargs):
        self._add_api_route(
            path=path, endpoint=endpoint, methods=http.GET, auth=auth, **kwargs
        )

    def add_post(self, path, endpoint, auth=False, **kwargs):
        self._add_api_route(
            path=path, endpoint=endpoint, methods=http.POST, auth=auth, **kwargs
        )

    def add_put(self, path, endpoint, auth=False, **kwargs):
        self._add_api_route(
            path=path, endpoint=endpoint, methods=http.PUT, auth=auth, **kwargs
        )

    def add_delete(self, path, endpoint, auth=False, **kwargs):
        self._add_api_route(
            path=path, endpoint=endpoint, methods=http.DELETE, auth=auth, **kwargs
        )

    def add_patch(self, path, endpoint, auth=False, **kwargs):
        self._add_api_route(
            path=path, endpoint=endpoint, methods=http.PATCH, auth=auth, **kwargs
        )


def generate_id(prefix: str) -> str:
    return f"{prefix}-{uuid5().hex}"


def get_openapi(app):
    def openapi():
        if not app.openapi_schema:
            app.openapi_schema = _get_openapi(
                title=app.title,
                version=app.version,
                openapi_version=app.openapi_version,
                description=app.description,
                terms_of_service=app.terms_of_service,
                contact=app.contact,
                license_info=app.license_info,
                routes=app.routes,
                tags=app.openapi_tags,
                servers=app.servers,
            )
            for _, method_item in app.openapi_schema.get("paths").items():
                for _, param in method_item.items():
                    responses = param.get("responses")
                    if "422" in responses:
                        del responses["422"]
        return app.openapi_schema

    return openapi
