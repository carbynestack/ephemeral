/*
 * Copyright (c) 2021 - for information on the respective copyright owner
 * see the NOTICE file and/or the repository https://github.com/carbynestack/ephemeral.
 *
 * SPDX-License-Identifier: Apache-2.0
 */
package io.carbynestack.ephemeral.client;

import java.util.List;
import java.util.UUID;
import lombok.AllArgsConstructor;
import lombok.Getter;
import lombok.NoArgsConstructor;

/**
 * Returned by {@link EphemeralClient} and {@link EphemeralMultiClient} in case of a successful
 * activation.
 */
@Getter
@NoArgsConstructor
@AllArgsConstructor
public class ActivationResult {

  /** The identifiers of the Amphora secrets returned by the function activation. */
  List<UUID> response;
}
