FROM gcr.io/distroless/static-debian11:nonroot
ENTRYPOINT ["/baton-notion"]
COPY baton-notion /