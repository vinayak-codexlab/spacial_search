import { describe, expect, test } from "vitest";

// Live API tests are deliberately opt-in: they must never pass by mocking the
// Go service. Start the service and set API_BASE_URL, e.g. http://127.0.0.1:3000.
const apiBaseURL = process.env.API_BASE_URL?.replace(/\/$/, "");
const api = apiBaseURL ? describe : describe.skip;

async function get(path) {
  const response = await fetch(`${apiBaseURL}${path}`);
  const body = await response.json();
  return { response, body };
}

api("GET /api/v1/listings/search", () => {
  test("returns a successful, non-cacheable response for valid coordinates", async () => {
    const { response, body } = await get("/api/v1/listings/search?lat=28&lng=77");

    expect(response.status).toBe(200);
    expect(response.headers.get("cache-control")).toBe("private, no-store");
    expect(body).toMatchObject({
      success: true,
      message: "Data fetched successfully",
      meta: {
        resolution_used: "h3_res9",
        zoom: 12,
        ring: 1,
        target_hexes_count: 7,
        result_limit: 250,
      },
    });
    expect(body.data).toEqual(expect.any(Array));
    expect(body.meta.count).toBe(body.data.length);
  });

  test.each([
    ["missing latitude", "?lng=77", "Latitude must be a valid number between -90 and 90"],
    ["non-numeric latitude", "?lat=NaN&lng=77", "Latitude must be a valid number between -90 and 90"],
    ["latitude out of range", "?lat=91&lng=77", "Latitude must be a valid number between -90 and 90"],
    ["missing longitude", "?lat=28", "Longitude must be a valid number between -180 and 180"],
    ["longitude out of range", "?lat=28&lng=181", "Longitude must be a valid number between -180 and 180"],
    ["invalid zoom", "?lat=28&lng=77&zoom=23", "Zoom must be an integer between 0 and 22"],
    ["fractional ring", "?lat=28&lng=77&ring=1.5", "Ring must be an integer between 0 and 25"],
    ["oversized ring", "?lat=28&lng=77&ring=26", "Ring must be an integer between 0 and 25"],
  ])("returns a clear 400 error for %s", async (_name, query, message) => {
    const { response, body } = await get(`/api/v1/listings/search${query}`);

    expect(response.status).toBe(400);
    expect(response.headers.get("cache-control")).toBe("private, no-store");
    expect(body).toEqual({ success: false, message });
  });
});
