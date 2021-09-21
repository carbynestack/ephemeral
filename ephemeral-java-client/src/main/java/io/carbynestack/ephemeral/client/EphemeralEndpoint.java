/*
 * Copyright (c) 2021 - for information on the respective copyright owner
 * see the NOTICE file and/or the repository https://github.com/carbynestack/ephemeral.
 *
 * SPDX-License-Identifier: Apache-2.0
 */
package io.carbynestack.ephemeral.client;

import io.carbynestack.httpclient.CsHttpClientException;
import io.vavr.control.Try;
import java.net.URI;
import lombok.Builder;
import lombok.NonNull;
import lombok.Value;
import org.apache.http.client.utils.URIBuilder;

/**
 * An Ephemeral service endpoint consisting of the Ephemeral service URI and the Knative application
 * name.
 */
@Value
public class EphemeralEndpoint {

  URI serviceUri;
  String application;

  /**
   * @param withServiceUri The URL of the Ephemeral service.
   * @param withApplication The name of the application to run the activation on.
   */
  @Builder(builderMethodName = "Builder")
  public EphemeralEndpoint(@NonNull URI withServiceUri, @NonNull String withApplication) {
    this.serviceUri = withServiceUri;
    this.application = withApplication;
  }

  /**
   * Returns the activation URL.
   *
   * @param compile If set, code provided as part of the {@link Activation} passed to {@link
   *     EphemeralClient#execute(Activation)} is compiled prior to execution.
   * @return The activation URL
   * @throws CsHttpClientException In case URI construction fails
   */
  public URI getActivationUri(boolean compile) throws CsHttpClientException {
    URIBuilder uriBuilder =
        new URIBuilder(serviceUri)
            .setHost(String.format("%s.%s", application, serviceUri.getHost()));
    if (compile) {
      uriBuilder.addParameter("compile", "true");
    }
    return Try.of(uriBuilder::build).getOrElseThrow(CsHttpClientException::new);
  }
}
