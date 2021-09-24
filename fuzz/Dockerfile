FROM golang:1.16-buster as builder
RUN set -eux; \
    apt-get update \
    && apt-get upgrade -y \
    && apt-get install -y \
    clang \
    curl \
    vim

RUN git clone https://github.com/fluxcd/pkg /workspace
RUN mkdir /workspace/fuzzing


RUN go get -u github.com/dvyukov/go-fuzz/go-fuzz@latest github.com/dvyukov/go-fuzz/go-fuzz-build@latest
RUN go install sigs.k8s.io/controller-runtime/tools/setup-envtest@latest
RUN go get github.com/AdaLogics/go-fuzz-headers

RUN go get golang.org/x/sync

WORKDIR /workspace/fuzzing
COPY fuzz.go /workspace/fuzzing/
COPY conditions_fuzzer.go /workspace/runtime/conditions/
COPY tls_fuzzer.go /workspace/runtime/tls/
RUN cd /workspace/fuzzing && go mod init fuzzing && go mod tidy && go mod download && go mod tidy
RUN go mod download github.com/dvyukov/go-fuzz


# Build the fuzzers
RUN mkdir /fuzzers

RUN go-fuzz-build -libfuzzer -func=FuzzUntar\
    && clang -o /fuzzers/FuzzUntar reflect-fuzz.a \
    -fsanitize=fuzzer
RUN go-fuzz-build -libfuzzer -func=FuzzLibGit2Error\
    && clang -o /fuzzers/FuzzLibGit2Error reflect-fuzz.a \
    -fsanitize=fuzzer
RUN go-fuzz-build -libfuzzer -func=FuzzEventInfof\
    && clang -o /fuzzers/FuzzEventInfof reflect-fuzz.a \
    -fsanitize=fuzzer

WORKDIR /workspace/runtime/tls
RUN go get github.com/AdaLogics/go-fuzz-headers
RUN go get github.com/dvyukov/go-fuzz/go-fuzz-dep
RUN go-fuzz-build -libfuzzer -func=FuzzTlsConfig\
    && clang -o /fuzzers/FuzzTlsConfig reflect-fuzz.a \
    -fsanitize=fuzzer

WORKDIR /workspace/runtime/conditions
RUN go-fuzz-build -libfuzzer -func=FuzzGetterConditions\
    && clang -o /fuzzers/FuzzGetterConditions reflect-fuzz.a \
    -fsanitize=fuzzer
RUN go-fuzz-build -libfuzzer -func=FuzzConditionsMatch\
    && clang -o /fuzzers/FuzzConditionsMatch reflect-fuzz.a \
    -fsanitize=fuzzer
RUN go-fuzz-build -libfuzzer -func=FuzzPatchApply\
    && clang -o /fuzzers/FuzzPatchApply reflect-fuzz.a \
    -fsanitize=fuzzer
RUN go-fuzz-build -libfuzzer -func=FuzzConditionsUnstructured\
    && clang -o /fuzzers/FuzzConditionsUnstructured reflect-fuzz.a \
    -fsanitize=fuzzer

#RUN /fuzzers/FuzzLibGit2Error