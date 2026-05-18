FROM golang:1.26.3-bookworm@sha256:b09e568dcf2a1ff3ce09a230ea234193fc014dc195472fe63316e50238453d96 as build

ARG GOFLAGS=""
ENV GOFLAGS=$GOFLAGS

WORKDIR /build

COPY go.mod .
COPY go.sum .
RUN go mod download
COPY . .

RUN go build ./trillian/ctfe/ct_server

FROM gcr.io/distroless/base-debian12@sha256:9dce90e688a57e59ce473ff7bc4c80bc8fe52d2303b4d99b44f297310bbd2210

COPY --from=build /build/ct_server /

ENTRYPOINT ["/ct_server"]
