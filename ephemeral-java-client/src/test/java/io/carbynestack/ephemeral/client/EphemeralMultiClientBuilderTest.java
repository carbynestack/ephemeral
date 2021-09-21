/*
 * Copyright (c) 2021 - for information on the respective copyright owner
 * see the NOTICE file and/or the repository https://github.com/carbynestack/ephemeral.
 *
 * SPDX-License-Identifier: Apache-2.0
 */
package io.carbynestack.ephemeral.client;

import static org.hamcrest.MatcherAssert.assertThat;
import static org.mockito.ArgumentMatchers.any;
import static org.mockito.ArgumentMatchers.anyBoolean;
import static org.mockito.Mockito.*;

import io.carbynestack.httpclient.CsHttpClientException;
import java.io.File;
import java.io.IOException;
import java.net.URI;
import java.util.Collections;
import java.util.List;
import java.util.stream.Collectors;
import java.util.stream.Stream;
import org.hamcrest.CoreMatchers;
import org.junit.Test;
import org.junit.runner.RunWith;
import org.mockito.Mock;
import org.mockito.junit.MockitoJUnitRunner;

@RunWith(MockitoJUnitRunner.class)
public class EphemeralMultiClientBuilderTest {

  private static final List<EphemeralEndpoint> ENDPOINTS =
      Stream.of("https://testUri:80", "https://testUri:180")
          .map(
              url ->
                  EphemeralEndpoint.Builder()
                      .withServiceUri(URI.create(url))
                      .withApplication("test")
                      .build())
          .collect(Collectors.toList());

  @Mock private EphemeralClient.EphemeralClientBuilder ephemeralClientBuilder;

  @Test
  public void givenEndpointList_whenBuilding_createsClientWithCorrectEndpoints()
      throws CsHttpClientException {
    EphemeralMultiClient client =
        new EphemeralMultiClient.Builder().withEndpoints(ENDPOINTS).build();
    assertThat(
        client.getEphemeralEndpoints(),
        CoreMatchers.hasItems(ENDPOINTS.toArray(new EphemeralEndpoint[0])));
  }

  @Test
  public void givenEndpointIndividualEndpoints_whenBuilding_createsClientWithCorrectEndpoints()
      throws CsHttpClientException {
    EphemeralMultiClient.Builder builder = new EphemeralMultiClient.Builder();
    ENDPOINTS.forEach(builder::withEndpoint);
    EphemeralMultiClient client = builder.build();
    assertThat(
        client.getEphemeralEndpoints(),
        CoreMatchers.hasItems(ENDPOINTS.toArray(new EphemeralEndpoint[0])));
  }

  @Test
  public void
      givenSslCertificateValidationDisabledOnBuilder_whenBuilding_createsUnderlyingClientsWithSslCertificateValidationDisabled()
          throws CsHttpClientException {
    when(ephemeralClientBuilder.withEndpoint(any())).thenReturn(ephemeralClientBuilder);
    when(ephemeralClientBuilder.withoutSslValidation(anyBoolean()))
        .thenReturn(ephemeralClientBuilder);
    when(ephemeralClientBuilder.withTrustedCertificates(any())).thenReturn(ephemeralClientBuilder);
    new EphemeralMultiClient.Builder()
        .withEphemeralClientBuilder(ephemeralClientBuilder)
        .withEndpoints(ENDPOINTS)
        .withSslCertificateValidation(false)
        .build();
    verify(ephemeralClientBuilder, times(2)).withoutSslValidation(true);
  }

  @Test
  public void
      givenTrustedCertificateProvidedToBuilder_whenBuilding_createsUnderlyingClientsWithCertificatesAdded()
          throws IOException {
    File cert = File.createTempFile("test", ".pem");
    when(ephemeralClientBuilder.withEndpoint(any())).thenReturn(ephemeralClientBuilder);
    when(ephemeralClientBuilder.withoutSslValidation(anyBoolean()))
        .thenReturn(ephemeralClientBuilder);
    when(ephemeralClientBuilder.withTrustedCertificates(any())).thenReturn(ephemeralClientBuilder);
    new EphemeralMultiClient.Builder()
        .withEphemeralClientBuilder(ephemeralClientBuilder)
        .withEndpoints(ENDPOINTS)
        .withTrustedCertificate(cert)
        .build();
    verify(ephemeralClientBuilder, times(2))
        .withTrustedCertificates(Collections.singletonList(cert));
  }
}
