// @ts-check
// Cynhyrchwyd y ffeil hon yn awtomatig. PEIDIWCH Â MODIWL
// This file is automatically generated. DO NOT EDIT

import {Call} from "@wailsio/runtime";

import {Person} from "./models.js";

/**
 * @param {Person} person
 * @returns {Promise<string>}
 */
export function Greet(person) {
    return Call.ByID(1675906978, person);
}
