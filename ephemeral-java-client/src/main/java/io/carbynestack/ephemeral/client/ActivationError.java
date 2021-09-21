/*
 * Copyright (c) 2021 - for information on the respective copyright owner
 * see the NOTICE file and/or the repository https://github.com/carbynestack/ephemeral.
 *
 * SPDX-License-Identifier: Apache-2.0
 */
package io.carbynestack.ephemeral.client;

import lombok.Getter;
import lombok.Setter;
import lombok.experimental.Accessors;

/**
 * Error returned by {@link EphemeralClient} and {@link EphemeralMultiClient} in case an error
 * occurs while activating a function.
 */
@Getter
@Setter
@Accessors(chain = true)
public class ActivationError {

  /** An HTTP status code. */
  Integer responseCode;

  /** A human readable error message. */
  String message;
}
