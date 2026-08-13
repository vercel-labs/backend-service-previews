import logging
import os
import uuid
from typing import Literal

import httpx
from fastapi import FastAPI, HTTPException
from pydantic import BaseModel, Field, ValidationError

logger = logging.getLogger(__name__)

app = FastAPI(title="Preview Checkout Service", version="1.0.0")

PRODUCTS = {
    "field-notes": {"name": "Field Notes", "unit_price_cents": 1800},
    "desk-lamp": {"name": "Signal Lamp", "unit_price_cents": 8400},
    "travel-mug": {"name": "Transit Mug", "unit_price_cents": 3200},
}


class QuoteRequest(BaseModel):
    product_id: Literal["field-notes", "desk-lamp", "travel-mug"]
    quantity: int = Field(ge=1)
    customer_tier: Literal["standard", "vip"]
    coupon: str = Field(default="", max_length=32)


class ReservationResponse(BaseModel):
    reservation_id: str
    reservation_status: Literal["reserved"]
    reserved_until: str
    reservation_ttl_seconds: int
    product_id: Literal["field-notes", "desk-lamp", "travel-mug"]
    quantity: int = Field(ge=1)
    service: dict[str, str]


def first_env(*names: str, fallback: str) -> str:
    for name in names:
        if value := os.getenv(name, "").strip():
            return value
    return fallback


def service_identity() -> dict[str, str]:
    environment = first_env("VERCEL_ENV", fallback="local")
    source_fallback = "local" if environment in {"local", "development"} else "CLI deployment"
    commit = first_env("VERCEL_GIT_COMMIT_SHA", fallback=source_fallback)
    return {
        "service": "checkout",
        "runtime": "Python + FastAPI",
        "environment": environment,
        "branch": first_env("VERCEL_GIT_COMMIT_REF", fallback=source_fallback),
        "commit": commit[:7] if commit != source_fallback and len(commit) >= 7 else commit,
    }


@app.get("/api/v1/checkout/health")
async def health() -> dict[str, object]:
    return {"status": "healthy", **service_identity()}


@app.post("/api/v1/checkout/quote")
async def create_quote(request: QuoteRequest) -> dict[str, object]:
    if request.coupon and request.coupon.upper() != "PREVIEW10":
        raise HTTPException(status_code=422, detail="Use PREVIEW10 or leave the coupon empty.")

    reservations_url = os.environ["RESERVATIONS_SERVICE_URL"].rstrip("/")

    try:
        async with httpx.AsyncClient(timeout=5.0) as client:
            response = await client.post(
                f"{reservations_url}/internal/reserve",
                json=request.model_dump(include={"product_id", "quantity"}),
            )
            if response.status_code == 409:
                reservation_error = response.json()
                raise HTTPException(
                    status_code=409,
                    detail={
                        "message": "There is not enough stock to reserve every item in your cart.",
                        "suggested_quantity": reservation_error["suggested_quantity"],
                    },
                )
            response.raise_for_status()
            reservation = ReservationResponse.model_validate(response.json())
            if reservation.product_id != request.product_id or reservation.quantity != request.quantity:
                logger.error(
                    "Reservation service returned a different product or quantity: %s",
                    reservation.model_dump(),
                )
                raise HTTPException(
                    status_code=502,
                    detail="The reservation service returned an incompatible response.",
                )
    except ValidationError as error:
        logger.error("Reservation service contract mismatch: %s", error)
        raise HTTPException(
            status_code=502,
            detail="The reservation service returned an incompatible response.",
        ) from error
    except (httpx.HTTPError, ValueError) as error:
        raise HTTPException(status_code=502, detail="The reservation preview service is unavailable.") from error

    product = PRODUCTS[request.product_id]
    subtotal = product["unit_price_cents"] * request.quantity
    tier_discount = subtotal * 15 // 100 if request.customer_tier == "vip" else 0
    discounted_total = subtotal - tier_discount
    coupon_discount = round(discounted_total * 0.10) if request.coupon else 0

    return {
        "quote_id": str(uuid.uuid4())[:8],
        "reservation_id": reservation.reservation_id,
        "reservation_status": reservation.reservation_status,
        "reserved_until": reservation.reserved_until,
        "reservation_ttl_seconds": reservation.reservation_ttl_seconds,
        "product": product["name"],
        "quantity": request.quantity,
        "currency": "USD",
        "subtotal_cents": subtotal,
        "tier_discount_cents": tier_discount,
        "coupon_discount_cents": coupon_discount,
        "total_cents": discounted_total - coupon_discount,
        "services": [service_identity(), reservation.service],
    }
