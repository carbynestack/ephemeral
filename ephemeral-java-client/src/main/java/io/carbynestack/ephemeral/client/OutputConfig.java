/*
 * Copyright (c) 2021 - for information on the respective copyright owner
 * see the NOTICE file and/or the repository https://github.com/carbynestack/ephemeral.
 *
 * SPDX-License-Identifier: Apache-2.0
 */
package io.carbynestack.ephemeral.client;

import lombok.AccessLevel;
import lombok.AllArgsConstructor;
import lombok.Data;
import lombok.NoArgsConstructor;

/** Specifies details of a function invocation. */
@Data
@AllArgsConstructor(access = AccessLevel.PRIVATE)
@NoArgsConstructor(access = AccessLevel.PRIVATE)
public final class OutputConfig {

  /** Configuration for functions that create Amphora secrets. */
  public static final OutputConfig AMPHORA_SECRET = new OutputConfig("AMPHORASECRET");

  /** The result type of a function activation. */
  private String type;
}
