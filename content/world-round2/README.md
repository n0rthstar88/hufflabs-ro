# World round 2: vending currency and random option drops

## Vending currency

Using Vending first shows a small pick window: **Zeny** or **HuffLabs Coin**. The shop then opens as
usual; a coin shop's title starts with `[HuffLabs Coin]` and buyers are told every price is in coins
(the client still prints "z").

Idea from Lilith/Easycore's "Extended Vending" (rathena.org), rewritten because the download:
- stored the currency item in a 16-bit field (our coin is 2600000, so it would have become another item),
- paid the seller once per matching stack in the buyer's inventory but took payment from one stack (duplication),
- built the sale-report mail from an uninitialised buffer,
- accepted any item id the client sent as the currency.

This version (`rathena.patch`, made by `vending-edit.py`): the shop state is set only after the pick
(closing the window leaves nothing half-open); coin payments are checked in full first (enough unbound,
unequipped coins; seller has room and weight), the seller gets a fresh stack, payment comes out of as many
stacks as needed; no tax on coins; shops left open offline keep their currency (`vendings.hlro_currency`).
Settings: `vending_currency_item` (2600000) and `vending_zeny_item` (2600200, the "Zeny" entry, an
untradeable placeholder in content/coin-shop). Set `vending_currency_item: 0` to go back to plain shops.

## Random option drops

`convert-randopt.py` turns the 2017 "Random Option drops" download (ado-3, txt) into YAML:
- 6 groups (`ADO_WEP`, `ADO_EQARM`, `ADO_EQSH`, `ADO_EQGAR`, `ADO_EQFOOT`, `ADO_EQACC`, ids 1000-1005):
  one guaranteed option weighted like the old first column, plus up to two random extras about as likely
  as the old 2- and 3-option rows; values are the old min..max.
- 792 monsters: 1,463 existing drops get a group (official rate kept), 251 drops added; 63 skipped
  because those monsters already have 10 drops.
- `mob_db_randopt.yml` is loaded through the Footer of content/leveling's `db/import/mob_db.yml`.
- Not in the public repo: `item_randomopt_group.yml` and `mob_db_randopt.yml` are converted from the
  download, so build them with `convert-randopt.py <rathena dir> <group txt> <mob_drop txt> server/db/import`
  before running `deploy.sh`.
