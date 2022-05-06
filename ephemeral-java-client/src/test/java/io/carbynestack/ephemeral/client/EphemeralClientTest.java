/*
 * Copyright (c) 2021 - for information on the respective copyright owner
 * see the NOTICE file and/or the repository https://github.com/carbynestack/ephemeral.
 *
 * SPDX-License-Identifier: Apache-2.0
 */
package io.carbynestack.ephemeral.client;

import static org.hamcrest.CoreMatchers.containsString;
import static org.hamcrest.CoreMatchers.equalTo;
import static org.hamcrest.CoreMatchers.is;
import static org.hamcrest.MatcherAssert.assertThat;
import static org.junit.Assert.assertEquals;
import static org.junit.Assert.assertThrows;
import static org.mockito.ArgumentMatchers.any;
import static org.mockito.Mockito.when;

import io.carbynestack.httpclient.BearerTokenUtils;
import io.carbynestack.httpclient.CsHttpClient;
import io.carbynestack.httpclient.CsHttpClientException;
import io.carbynestack.httpclient.CsResponseEntity;
import io.vavr.control.Either;
import io.vavr.control.Option;
import io.vavr.control.Try;
import java.net.URI;
import java.util.Collections;
import java.util.List;
import java.util.UUID;
import org.apache.commons.lang3.RandomStringUtils;
import org.apache.http.Header;
import org.junit.Before;
import org.junit.Test;
import org.junit.runner.RunWith;
import org.mockito.ArgumentCaptor;
import org.mockito.Mock;
import org.mockito.junit.MockitoJUnitRunner;

@RunWith(MockitoJUnitRunner.class)
public class EphemeralClientTest {

  private static final String APPLICATION = "app";
  private static final URI TEST_URI = Try.of(() -> new URI("http://localhost")).get();
  private final EphemeralEndpoint endpoint = new EphemeralEndpoint(TEST_URI, APPLICATION);

  @Mock private CsHttpClient<String> specsHttpClientMock;
  private EphemeralClient client;

  @Before
  public void setUp() throws CsHttpClientException {
    client = new EphemeralClient(endpoint, specsHttpClientMock, Option.none());
  }

  @Test
  public void givenServiceUrlIsNull_whenCreateClient_thenThrowException() {
    CsHttpClientException sce =
        assertThrows(CsHttpClientException.class, () -> EphemeralClient.Builder().build());
    assertThat(sce.getMessage(), containsString("Endpoint must not be null."));
  }

  @Test
  public void givenSuccessful_whenExecuteProgram_thenReturnResult() throws CsHttpClientException {
    ActivationResult result = new ActivationResult(Collections.singletonList(UUID.randomUUID()));
    Activation activation = Activation.builder().build();
    when(specsHttpClientMock.postForEntity(
            endpoint.getActivationUri(activation.getCode() != null),
            Collections.emptyList(),
            activation,
            ActivationResult.class))
        .thenReturn(CsResponseEntity.success(200, result));
    Either<ActivationError, ActivationResult> eitherFuture = client.execute(activation);
    assertThat(eitherFuture.get(), equalTo(result));
  }

  @Test
  public void givenServiceRespondsUnsuccessful_whenExecuteProgram_thenReturnFailureCode()
      throws CsHttpClientException {
    int httpFailureCode = 404;
    String errMessage = "some failure";
    Activation activation = Activation.builder().build();
    when(specsHttpClientMock.postForEntity(
            endpoint.getActivationUri(activation.getCode() != null),
            Collections.emptyList(),
            activation,
            ActivationResult.class))
        .thenReturn(CsResponseEntity.failed(httpFailureCode, errMessage));
    Either<ActivationError, ActivationResult> result = client.execute(Activation.builder().build());
    assertThat(result.getLeft().responseCode, equalTo(httpFailureCode));
    assertThat(result.getLeft().message, equalTo(errMessage));
  }

  @Test
  public void
      givenBearerTokenConfigured_whenExecuteProgram_thenSpecsClientIsInvokedWithAuthorizationHeader()
          throws CsHttpClientException {
    String token = RandomStringUtils.randomAlphanumeric(20);
    EphemeralClient clientWithToken =
        new EphemeralClient(endpoint, specsHttpClientMock, Option.some(token));
    ActivationResult result = new ActivationResult(Collections.singletonList(UUID.randomUUID()));
    Activation activation = Activation.builder().build();
    ArgumentCaptor<List<Header>> headersCaptor = ArgumentCaptor.forClass(List.class);
    when(specsHttpClientMock.postForEntity(any(), headersCaptor.capture(), any(), any()))
        .thenReturn(CsResponseEntity.success(200, result));
    clientWithToken.execute(activation);
    List<Header> headers = headersCaptor.getValue();
    assertThat("No header has been supplied", headers.size(), is(1));
    Header header = headers.get(0);
    Header expected = BearerTokenUtils.createBearerToken(token);
    assertEquals(expected.getName(), header.getName());
    assertEquals(expected.getValue(), header.getValue());
  }
}
