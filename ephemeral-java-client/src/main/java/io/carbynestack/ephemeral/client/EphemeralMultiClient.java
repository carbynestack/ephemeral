/*
 * Copyright (c) 2021 - for information on the respective copyright owner
 * see the NOTICE file and/or the repository https://github.com/carbynestack/ephemeral.
 *
 * SPDX-License-Identifier: Apache-2.0
 */
package io.carbynestack.ephemeral.client;

import io.carbynestack.ephemeral.client.EphemeralClient.EphemeralClientBuilder;
import io.carbynestack.httpclient.CsHttpClientException;
import io.vavr.collection.Seq;
import io.vavr.collection.Stream;
import io.vavr.concurrent.Future;
import io.vavr.control.Either;
import io.vavr.control.Option;
import io.vavr.control.Try;
import java.io.File;
import java.util.ArrayList;
import java.util.List;
import java.util.UUID;
import java.util.stream.Collectors;
import lombok.NonNull;
import lombok.extern.slf4j.Slf4j;

/**
 * A client for consuming the Ephemeral services of a Carbyne Stack Virtual Cloud.
 *
 * <p>The client interacts with all ephemeral backend services in a Carbyne Stack Virtual Cloud in
 * order to execute MPC programs.
 *
 * <p>The methods of this client are all asynchronous. The returned future completes normally with
 * either the a list of the requested domain objects or the first http status code in case one or
 * more invocations of the backend Ephemeral services fails.
 *
 * <p>In case a low-level error occurs during service invocation the future completes exceptionally.
 * See {@link EphemeralClient} for the exceptions potentially thrown.
 */
@Slf4j
public class EphemeralMultiClient {

  private final List<EphemeralClient> clients;

  EphemeralMultiClient(final List<EphemeralClient> clients) {
    this.clients = clients;
  }

  /**
   * The endpoints this client talks to.
   *
   * @return The list of endpoints
   */
  public List<EphemeralEndpoint> getEphemeralEndpoints() {
    return clients.stream().map(EphemeralClient::getEndpoint).collect(Collectors.toList());
  }

  /**
   * Compiles the given source code and triggers its execution on all Ephemeral endpoints this
   * client talks to.
   *
   * @param code The MPC source code of the function to be invoked.
   * @param inputSecretIds The UUIDs of the Amphora secrets used as inputs to the function
   *     execution.
   * @return A future completing normally either with a list of Amphora secret identifiers or an
   *     http error code if the execution failed on server side and exceptionally in case an error
   *     occurs on the network or representation layer
   */
  public Future<Either<ActivationError, List<ActivationResult>>> execute(
      @NonNull String code, @NonNull List<UUID> inputSecretIds) {
    return execute(code, UUID.randomUUID(), inputSecretIds);
  }

  /**
   * Compiles the given source code and triggers its execution on all Ephemeral endpoints this
   * client talks to.
   *
   * @param code The MPC source code of the function to be invoked.
   * @param gameId The UUID specifying the Ephemeral Execution ID used to correlate invocations
   *     across the virtual cloud providers.
   * @param inputSecretIds The UUIDs of the Amphora secrets used as inputs to the function
   *     execution.
   * @return A future completing normally either with a list of Amphora secret identifiers or an
   *     http error code if the execution failed on server side and exceptionally in case an error
   *     occurs on the network or representation layer
   */
  public Future<Either<ActivationError, List<ActivationResult>>> execute(
      @NonNull String code, @NonNull UUID gameId, @NonNull List<UUID> inputSecretIds) {
    if (log.isDebugEnabled()) {
      log.debug(
          "Invoking ephemeral services at {} and game {}",
          clients.stream().map(EphemeralClient::getEndpoint),
          gameId);
    }
    Seq<Future<Either<ActivationError, ActivationResult>>> invocations =
        Stream.ofAll(clients)
            .zipWithIndex()
            .map(
                t -> {
                  Activation.ActivationBuilder activationBuilder =
                      new Activation.ActivationBuilder()
                          .gameId(gameId.toString())
                          .amphoraParams(
                              Stream.ofAll(inputSecretIds)
                                  .map(UUID::toString)
                                  .toJavaArray(String[]::new))
                          .code(code);
                  Activation activation = activationBuilder.build();
                  if (log.isDebugEnabled()) {
                    log.debug(
                        "Activation for ephemeral service at {} is {}",
                        clients.get(t._2).getEndpoint(),
                        activation);
                  }
                  return t._1.execute(activation);
                });
    return Future.sequence(invocations)
        .andThen(a -> a.forEach(e -> log.debug("Results for game {} are {}", gameId, e.asJava())))
        .map(
            s ->
                s.foldLeft(
                    Either.right(new ArrayList<>()),
                    (i, j) -> {
                      if (i.isLeft()) {
                        return i;
                      }
                      if (j.isLeft()) {
                        return Either.left(j.getLeft());
                      } else {
                        return i.map(
                            l -> {
                              l.add(j.get());
                              return l;
                            });
                      }
                    }));
  }

  /** Provides the bearer token used for authentication. */
  public interface BearerTokenProvider {

    /**
     * Returns the bearer token for an Ephemeral endpoint.
     *
     * @param endpoint The endpoint of the Ephemeral service for which the token is requested.
     * @return The token
     */
    String getBearerToken(EphemeralEndpoint endpoint);
  }

  /** Builder class to create a new {@link EphemeralMultiClient}. */
  public static class Builder {

    private List<EphemeralEndpoint> endpoints;
    private final List<File> trustedCertificates;
    private boolean sslValidationEnabled = true;
    private Option<BearerTokenProvider> bearerTokenProvider;
    private EphemeralClientBuilder ephemeralClientBuilder = EphemeralClient.Builder();

    public Builder() {
      this.endpoints = new ArrayList<>();
      this.trustedCertificates = new ArrayList<>();
      bearerTokenProvider = Option.none();
    }

    /**
     * Adds an Ephemeral service endpoint to the list of endpoints, the client should communicate
     * with.
     *
     * @param endpoint Endpoint of a backend Ephemeral Service
     */
    public Builder withEndpoint(EphemeralEndpoint endpoint) {
      this.endpoints.add(endpoint);
      return this;
    }

    /**
     * The client will be initialized to communicate with the given endpoints. All endpoints that
     * have been added before will be replaced. To add additional endpoints use {@link
     * #withEndpoint(EphemeralEndpoint)}.
     *
     * @param endpoints A List of endpoints which will be used to communicate with.
     */
    public Builder withEndpoints(@NonNull List<EphemeralEndpoint> endpoints) {
      this.endpoints = new ArrayList<>(endpoints);
      return this;
    }

    /**
     * Controls whether SSL certificate validation is performed.
     *
     * <p>
     *
     * <p><b>WARNING</b><br>
     * Please be aware, that disabling validation leads to insecure web connections and is meant to
     * be used in a local test setup only. Using this option in a productive environment is
     * explicitly <u>not recommended</u>.
     *
     * @param enabled <tt>true</tt>, in case SSL certificate validation should happen,
     *     <tt>false</tt> otherwise
     */
    public Builder withSslCertificateValidation(boolean enabled) {
      this.sslValidationEnabled = enabled;
      return this;
    }

    /**
     * Adds a certificate (.pem) to the trust store.<br>
     * This allows tls secured communication with services that do not have a certificate issued by
     * an official CA (certificate authority).
     *
     * @param trustedCertificate Public certificate.
     */
    public Builder withTrustedCertificate(File trustedCertificate) {
      this.trustedCertificates.add(trustedCertificate);
      return this;
    }

    /**
     * Sets a provider for getting a backend specific bearer token that is injected as an
     * authorization header to REST HTTP calls emitted by the client.
     *
     * @param bearerTokenProvider Provider for backend specific bearer token
     */
    public Builder withBearerTokenProvider(BearerTokenProvider bearerTokenProvider) {
      this.bearerTokenProvider = Option.of(bearerTokenProvider);
      return this;
    }

    protected Builder withEphemeralClientBuilder(EphemeralClientBuilder ephemeralClientBuilder) {
      this.ephemeralClientBuilder = ephemeralClientBuilder;
      return this;
    }

    /**
     * Builds and returns a new {@link EphemeralMultiClient} according to the given configuration.
     *
     * @throws CsHttpClientException If the client could not be instantiated.
     */
    public EphemeralMultiClient build() throws CsHttpClientException {
      if (this.endpoints == null || this.endpoints.isEmpty()) {
        throw new IllegalArgumentException(
            "At least one Ephemeral service endpoint has to be provided.");
      }
      List<EphemeralClient> clients =
          Try.sequence(
                  this.endpoints.stream()
                      .map(
                          endpoint ->
                              Try.of(
                                  () -> {
                                    EphemeralClientBuilder b =
                                        ephemeralClientBuilder
                                            .withEndpoint(endpoint)
                                            .withoutSslValidation(!this.sslValidationEnabled)
                                            .withTrustedCertificates(this.trustedCertificates);
                                    this.bearerTokenProvider.forEach(
                                        p ->
                                            b.withBearerToken(
                                                Option.of(p.getBearerToken(endpoint))));
                                    return b.build();
                                  }))
                      .collect(Collectors.toList()))
              .getOrElseThrow(CsHttpClientException::new)
              .toJavaList();
      return new EphemeralMultiClient(clients);
    }
  }
}
