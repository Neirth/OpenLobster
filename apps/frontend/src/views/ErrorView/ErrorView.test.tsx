// Copyright (c) OpenLobster contributors. See LICENSE for details.
/* eslint-disable no-undef, @typescript-eslint/no-explicit-any */

import { describe, it, expect, vi } from "vitest";
import { render } from "@solidjs/testing-library";

vi.mock("../../App", () => ({
  t: (key: string) => key,
}));

import ErrorView, { Error400, Error403, Error404, Error500 } from "./ErrorView";

describe("ErrorView Component", () => {
  it("renders with error-view class when not fullscreen", () => {
    const { container } = render(() => <ErrorView code={400} fullscreen={false} />);
    expect(container.querySelector(".error-view")).toBeTruthy();
    expect(container.querySelector(".error-view--fullscreen")).toBeNull();
  });

  it("renders with error-view--fullscreen class when fullscreen", () => {
    const { container } = render(() => <ErrorView code={404} fullscreen={true} />);
    expect(container.querySelector(".error-view--fullscreen")).toBeTruthy();
    expect(container.querySelector(".error-view")).toBeNull();
  });

  it("defaults fullscreen to false when not provided", () => {
    const { container } = render(() => <ErrorView code={400} />);
    expect(container.querySelector(".error-view")).toBeTruthy();
    expect(container.querySelector(".error-view--fullscreen")).toBeNull();
  });

  it("renders the error code title translation key", () => {
    const { getByText } = render(() => <ErrorView code={404} fullscreen={true} />);
    expect(getByText("error.404.title")).toBeTruthy();
  });

  it("renders the error message translation key", () => {
    const { getByText } = render(() => <ErrorView code={404} fullscreen={true} />);
    expect(getByText("error.404.message")).toBeTruthy();
  });

  it("renders error-content container", () => {
    const { container } = render(() => <ErrorView code={500} fullscreen={true} />);
    expect(container.querySelector(".error-content")).toBeTruthy();
  });

  it("renders error-code heading element", () => {
    const { container } = render(() => <ErrorView code={500} />);
    expect(container.querySelector(".error-code")).toBeTruthy();
  });

  it("renders error-message paragraph element", () => {
    const { container } = render(() => <ErrorView code={500} />);
    expect(container.querySelector(".error-message")).toBeTruthy();
  });

  it("uses correct translation key for 400", () => {
    const { getByText } = render(() => <ErrorView code={400} />);
    expect(getByText("error.400.title")).toBeTruthy();
    expect(getByText("error.400.message")).toBeTruthy();
  });

  it("uses correct translation key for 403", () => {
    const { getByText } = render(() => <ErrorView code={403} />);
    expect(getByText("error.403.title")).toBeTruthy();
    expect(getByText("error.403.message")).toBeTruthy();
  });

  it("uses correct translation key for 500", () => {
    const { getByText } = render(() => <ErrorView code={500} />);
    expect(getByText("error.500.title")).toBeTruthy();
    expect(getByText("error.500.message")).toBeTruthy();
  });
});

describe("Error400 export", () => {
  it("renders without fullscreen class", () => {
    const { container } = render(() => <Error400 />);
    expect(container.querySelector(".error-view")).toBeTruthy();
    expect(container.querySelector(".error-view--fullscreen")).toBeNull();
  });

  it("renders correct translation key", () => {
    const { getByText } = render(() => <Error400 />);
    expect(getByText("error.400.title")).toBeTruthy();
  });
});

describe("Error403 export", () => {
  it("renders without fullscreen class", () => {
    const { container } = render(() => <Error403 />);
    expect(container.querySelector(".error-view")).toBeTruthy();
    expect(container.querySelector(".error-view--fullscreen")).toBeNull();
  });

  it("renders correct translation key", () => {
    const { getByText } = render(() => <Error403 />);
    expect(getByText("error.403.title")).toBeTruthy();
  });
});

describe("Error404 export", () => {
  it("renders with fullscreen class", () => {
    const { container } = render(() => <Error404 />);
    expect(container.querySelector(".error-view--fullscreen")).toBeTruthy();
  });

  it("renders correct translation key", () => {
    const { getByText } = render(() => <Error404 />);
    expect(getByText("error.404.title")).toBeTruthy();
  });
});

describe("Error500 export", () => {
  it("renders with fullscreen class", () => {
    const { container } = render(() => <Error500 />);
    expect(container.querySelector(".error-view--fullscreen")).toBeTruthy();
  });

  it("renders correct translation key", () => {
    const { getByText } = render(() => <Error500 />);
    expect(getByText("error.500.title")).toBeTruthy();
  });
});
