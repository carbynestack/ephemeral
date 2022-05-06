/*
 * Copyright (c) 2021 - for information on the respective copyright owner
 * see the NOTICE file and/or the repository https://github.com/carbynestack/ephemeral.
 *
 * SPDX-License-Identifier: Apache-2.0
 */
package io.carbynestack.ephemeral.client;

import io.carbynestack.httpclient.BearerTokenUtils;
import io.carbynestack.httpclient.CsHttpClient;
import io.carbynestack.httpclient.CsHttpClientException;
import io.carbynestack.httpclient.CsResponseEntity;
import io.vavr.concurrent.Future;
import io.vavr.control.Either;
import io.vavr.control.Option;
import java.io.File;
import java.util.List;
import java.util.stream.Collectors;
import lombok.AccessLevel;
import lombok.Getter;
import lombok.NonNull;
import lombok.extern.slf4j.Slf4j;

/**
 * A client for the Carbyne Stack Ephemeral service.
 *
 * <p>The client interacts with a single ephemeral backend service in order to execute MPC programs.
 *
 * <p>The methods of this client are all asynchronous and return a {@link Future}. This future
 * completes normally with either the requested domain object or a http status code.
 *
 * <p>In case a low-level error occurs on the network or representation layer the future terminates
 * exceptionally.
 *
 * <p>The public API of this class is defensive, i.e., arguments are checked for validity and an
 * {@link IllegalArgumentException} is thrown in case of a contract violation.
 */
@Slf4j
public class EphemeralClient {

  @Getter(value = AccessLevel.PACKAGE)
  private final EphemeralEndpoint endpoint;

  private final CsHttpClient<String> csHttpClient;
  private final Option<String> bearerToken;

  @lombok.Builder(builderMethodName = "Builder")
  private EphemeralClient(
      @NonNull EphemeralEndpoint withEndpoint,
      List<File> withTrustedCertificates,
      boolean withoutSslValidation,
      Option<String> withBearerToken)
      throws CsHttpClientException {
    this(
        withEndpoint,
        CsHttpClient.<String>builder()
            .withFailureType(String.class)
            .withTrustedCertificates(withTrustedCertificates)
            .withoutSslValidation(withoutSslValidation)
            .build(),
        withBearerToken == null ? Option.none() : withBearerToken);
  }

  EphemeralClient(
      EphemeralEndpoint endpoint, CsHttpClient<String> csHttpClient, Option<String> bearerToken)
      throws CsHttpClientException {
    if (endpoint == null) {
      throw new CsHttpClientException("Endpoint must not be null.");
    }
    this.endpoint = endpoint;
    this.csHttpClient = csHttpClient;
    this.bearerToken = bearerToken;
  }

  /**
   * Triggers a program execution.
   *
   * @param activation {@link Activation} configuration used as input for the program activation.
   * @return Either a list of Amphora secret identifiers or an http error code if the execution
   *     failed on server side.
   * @throws CsHttpClientException In case an error occurs on the network or representation layer
   */
  public Either<ActivationError, ActivationResult> execute(@NonNull Activation activation)
      throws CsHttpClientException {
    CsResponseEntity<String, ActivationResult> responseEntity =
        csHttpClient.postForEntity(
            endpoint.getActivationUri(
                activation.getCode() != null && !activation.getCode().isEmpty()),
            bearerToken.map(BearerTokenUtils::createBearerToken).collect(Collectors.toList()),
            activation,
            ActivationResult.class);
    return responseEntity
        .getContent()
        .mapLeft(
            l ->
                new ActivationError()
                    .setMessage(l)
                    .setResponseCode(responseEntity.getHttpStatus()));
  }
}
