from fastapi.testclient import TestClient
import httpx

from main import app


client = TestClient(app)


def test_health_reports_local_service_identity() -> None:
    response = client.get("/api/v1/checkout/health")

    assert response.status_code == 200
    assert response.json()["service"] == "checkout"
    assert response.json()["environment"] == "local"


def test_identity_uses_vercel_git_source(monkeypatch) -> None:
    monkeypatch.setenv("VERCEL_ENV", "production")
    monkeypatch.setenv("VERCEL_GIT_COMMIT_REF", "release/stock-holds")
    monkeypatch.setenv("VERCEL_GIT_COMMIT_SHA", "abcdef1234567890")

    response = client.get("/api/v1/checkout/health")

    assert response.json()["branch"] == "release/stock-holds"
    assert response.json()["commit"] == "abcdef1"


def test_cli_deployment_reports_neutral_source(monkeypatch) -> None:
    monkeypatch.setenv("VERCEL_ENV", "production")
    monkeypatch.setenv("VERCEL_GIT_COMMIT_REF", "")
    monkeypatch.setenv("VERCEL_GIT_COMMIT_SHA", "")
    response = client.get("/api/v1/checkout/health")

    assert response.json()["branch"] == "CLI deployment"
    assert response.json()["commit"] == "CLI deployment"


def test_quote_request_rejects_unknown_customer_tier() -> None:
    response = client.post(
        "/api/v1/checkout/quote",
        json={
            "product_id": "desk-lamp",
            "quantity": 1,
            "customer_tier": "partner",
            "coupon": "",
        },
    )

    assert response.status_code == 422


def test_quote_reports_stock_rejection_from_reservations(monkeypatch) -> None:
    class FakeResponse:
        status_code = 409

        def json(self) -> dict[str, int]:
            return {"suggested_quantity": 3}

    class FakeClient:
        def __init__(self, timeout: float) -> None:
            self.timeout = timeout

        async def __aenter__(self):
            return self

        async def __aexit__(self, *_args) -> None:
            return None

        async def post(self, _url: str, json: dict[str, object]) -> FakeResponse:
            assert json["quantity"] == 6
            return FakeResponse()

    monkeypatch.setenv("RESERVATIONS_SERVICE_URL", "http://reservations.internal")
    monkeypatch.setattr(httpx, "AsyncClient", FakeClient)

    response = client.post(
        "/api/v1/checkout/quote",
        json={
            "product_id": "desk-lamp",
            "quantity": 6,
            "customer_tier": "standard",
            "coupon": "",
        },
    )

    assert response.status_code == 409
    assert response.json()["detail"] == {
        "message": "There is not enough stock to reserve every item in your cart.",
        "suggested_quantity": 3,
    }


def test_quote_returns_private_reservation(monkeypatch) -> None:
    class FakeResponse:
        status_code = 201

        def raise_for_status(self) -> None:
            return None

        def json(self) -> dict[str, object]:
            return {
                "reservation_id": "res_test",
                "reservation_status": "reserved",
                "reserved_until": "2026-08-12T10:15:00Z",
                "reservation_ttl_seconds": 900,
                "product_id": "field-notes",
                "quantity": 2,
                "service": {"service": "reservations"},
            }

    class FakeClient:
        def __init__(self, timeout: float) -> None:
            self.timeout = timeout

        async def __aenter__(self):
            return self

        async def __aexit__(self, *_args) -> None:
            return None

        async def post(self, url: str, json: dict[str, object]) -> FakeResponse:
            assert url == "http://reservations.internal/internal/reserve"
            assert json == {"product_id": "field-notes", "quantity": 2}
            return FakeResponse()

    monkeypatch.setenv("RESERVATIONS_SERVICE_URL", "http://reservations.internal")
    monkeypatch.setattr(httpx, "AsyncClient", FakeClient)

    response = client.post(
        "/api/v1/checkout/quote",
        json={
            "product_id": "field-notes",
            "quantity": 2,
            "customer_tier": "vip",
            "coupon": "PREVIEW10",
        },
    )

    assert response.status_code == 200
    assert response.json()["reservation_id"] == "res_test"
    assert response.json()["product"] == "Field Notes"
    assert response.json()["subtotal_cents"] == 3600
    assert response.json()["tier_discount_cents"] == 540
    assert response.json()["coupon_discount_cents"] == 306
    assert response.json()["total_cents"] == 2754


def test_quote_reports_reservation_contract_mismatch(monkeypatch) -> None:
    class FakeResponse:
        status_code = 201

        def raise_for_status(self) -> None:
            return None

        def json(self) -> dict[str, object]:
            return {
                "reservation_id": "res_test",
                "reservation_status": "reserved",
                "expires_at": "2026-08-12T10:15:00Z",
                "reservation_ttl_seconds": 900,
                "product_id": "field-notes",
                "quantity": 1,
                "service": {"service": "reservations"},
            }

    class FakeClient:
        def __init__(self, timeout: float) -> None:
            self.timeout = timeout

        async def __aenter__(self):
            return self

        async def __aexit__(self, *_args) -> None:
            return None

        async def post(self, _url: str, json: dict[str, object]) -> FakeResponse:
            return FakeResponse()

    monkeypatch.setenv("RESERVATIONS_SERVICE_URL", "http://reservations.internal")
    monkeypatch.setattr(httpx, "AsyncClient", FakeClient)

    response = client.post(
        "/api/v1/checkout/quote",
        json={
            "product_id": "field-notes",
            "quantity": 1,
            "customer_tier": "standard",
            "coupon": "",
        },
    )

    assert response.status_code == 502
    assert response.json()["detail"] == "The reservation service returned an incompatible response."
