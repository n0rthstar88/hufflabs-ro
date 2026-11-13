"""HuffLabs vending currency (idea from Lilith/Easycore's Extended Vending, rewritten).
usage: vending-edit.py <rathena src dir>"""
import sys
root = sys.argv[1]
def edit(rel, old, new):
    p = f"{root}/{rel}"; s = open(p).read()
    assert s.count(old) == 1, (rel, old[:70], s.count(old))
    open(p, "w").write(s.replace(old, new))

# settings
edit("custom/battle_config_struct.inc", "\tint32 miniboss_card_announce_color;\n",
     "\tint32 miniboss_card_announce_color;\n\tint32 vending_currency_item;\n\tint32 vending_zeny_item;\n")
edit("custom/battle_config_init.inc", '\t{ "miniboss_card_announce_color",       &battle_config.miniboss_card_announce_color,    0xF6FF00, 0,      0xFFFFFF,       },\n',
     '\t{ "miniboss_card_announce_color",       &battle_config.miniboss_card_announce_color,    0xF6FF00, 0,      0xFFFFFF,       },\n'
     '\t// HuffLabs: vending currency. 0 = plain zeny shops. zeny item = the "Zeny" entry in the currency menu.\n'
     '\t{ "vending_currency_item",              &battle_config.vending_currency_item,           2600000,  0,      INT32_MAX,      },\n'
     '\t{ "vending_zeny_item",                  &battle_config.vending_zeny_item,               2600200,  0,      INT32_MAX,      },\n')

# state
edit("map/pc.hpp", "\tuint16 vend_skill_lv;\n", "\tuint16 vend_skill_lv;\n\tt_itemid vend_currency; // HuffLabs: 0 = zeny, else the item this shop takes\n")
edit("map/buyingstore.hpp", "\tuint16 count; ///< Number of item in store\n", "\tuint16 count; ///< Number of item in store\n\tt_itemid currency; ///< HuffLabs: vending currency item (0 = zeny)\n")

# skill: ask for the currency first
edit("map/skills/merchant/skill_vending.cpp", '#include "map/pc.hpp"\n', '#include "map/pc.hpp"\n#include "map/vending.hpp"\n')
edit("map/skills/merchant/skill_vending.cpp", "\t\telse {\n\t\t\tint32 i = 0;\n\t\t\tsd->state.prevend = 1;",
     "\t\telse if (vending_currency_enabled()) {\n\t\t\t// HuffLabs: pick Zeny or the currency item first; the shop opens after the pick\n\t\t\tsd->vend_currency = 0;\n\t\t\tsd->vend_skill_lv = skill_lv;\n\t\t\tvending_currency_menu(*sd, skill_lv);\n\t\t}\n\t\telse {\n\t\t\tint32 i = 0;\n\t\t\tsd->vend_currency = 0;\n\t\t\tsd->state.prevend = 1;")

# client packets
edit("map/clif.cpp", "\t\tcase NC_MAGICDECOY:\n\t\t\tskill_magicdecoy(*sd,p->itemId);\n\t\t\tbreak;\n\t}\n\n\tclif_menuskill_clear(sd);",
     "\t\tcase NC_MAGICDECOY:\n\t\t\tskill_magicdecoy(*sd,p->itemId);\n\t\t\tbreak;\n\t\tcase MC_VENDING: // HuffLabs vending currency\n\t\t\tvending_currency_select(*sd, p->itemId);\n\t\t\tbreak;\n\t}\n\n\tclif_menuskill_clear(sd);")
edit("map/clif.cpp", "\tif( message[0] == '\\0' ) // invalid input\n\t\treturn;\n\n\tvending_openvending(*sd, message, data, len/8, nullptr);",
     "\tif( message[0] == '\\0' ) // invalid input\n\t\treturn;\n\n"
     "\t// HuffLabs: the title shows the currency, e.g. \"[HuffLabs Coin] Cheap cards\"\n"
     "\tif( sd->vend_currency != 0 ){\n\t\tchar title[MESSAGE_SIZE];\n\t\tstd::shared_ptr<item_data> cur = item_db.find( sd->vend_currency );\n"
     "\t\tsafesnprintf( title, sizeof( title ), \"[%s] %s\", cur != nullptr ? cur->ename.c_str() : \"?\", message );\n"
     "\t\tvending_openvending(*sd, title, data, len/8, nullptr);\n\t\treturn;\n\t}\n\n"
     "\tvending_openvending(*sd, message, data, len/8, nullptr);")

# vending.hpp declarations
edit("map/vending.hpp", "void vending_reopen( map_session_data& sd );\n",
     "void vending_reopen( map_session_data& sd );\n\n// HuffLabs vending currency\nbool vending_currency_enabled();\nvoid vending_currency_menu( map_session_data& sd, uint16 skill_lv );\nvoid vending_currency_select( map_session_data& sd, t_itemid nameid );\n")

# vending.cpp: buyer notice
edit("map/vending.cpp", "\tsd->vended_id = vsd->vender_id;  // register vending uid\n\n\tclif_vendinglist( *sd, *vsd );",
     "\tsd->vended_id = vsd->vender_id;  // register vending uid\n\n"
     "\t// HuffLabs: say which currency this shop takes (the client always shows \"z\")\n"
     "\tif( vsd->vend_currency != 0 ){\n\t\tstd::shared_ptr<item_data> cur = item_db.find( vsd->vend_currency );\n"
     "\t\tstd::string note = \"This shop sells for \" + ( cur != nullptr ? cur->ename : std::string( \"items\" ) ) + \": every price is in that item, not zeny.\";\n"
     "\t\tclif_messagecolor( sd, color_table[COLOR_CYAN], note.c_str(), false, SELF );\n\t}\n\n"
     "\tclif_vendinglist( *sd, *vsd );")

# purchase checks
edit("map/vending.cpp", "\t\tz += ((double)vsd->vending[j].value * (double)amount);\n\t\tif( z > (double)sd->status.zeny || z < 0. || z > (double)MAX_ZENY ) {",
     "\t\tz += ((double)vsd->vending[j].value * (double)amount);\n"
     "\t\tif( vsd->vend_currency != 0 ){ // HuffLabs: paid in items, checked in full before payment\n"
     "\t\t\tif( z < 0. || z > (double)INT32_MAX ) {\n\t\t\t\tclif_buyvending( *sd, idx, amount, PURCHASEMC_NO_ZENY );\n\t\t\t\treturn;\n\t\t\t}\n"
     "\t\t} else if( z > (double)sd->status.zeny || z < 0. || z > (double)MAX_ZENY ) {")
edit("map/vending.cpp", "\t\tif( z + (double)vsd->status.zeny > (double)MAX_ZENY ) {",
     "\t\tif( vsd->vend_currency == 0 && z + (double)vsd->status.zeny > (double)MAX_ZENY ) {")
edit("map/vending.cpp", "\tpc_payzeny(sd, (int32)z, LOG_TYPE_VENDING, vsd->status.char_id);\n\tachievement_update_objective(sd, AG_SPEND_ZENY, 1, (int32)z);\n\tz = vending_calc_tax(sd, z);\n\tpc_getzeny(vsd, (int32)z, LOG_TYPE_VENDING, sd->status.char_id);\n",
     "\tif( vsd->vend_currency != 0 ){\n\t\tif( !vending_pay_with_item( *sd, *vsd, (int32)z ) )\n\t\t\treturn;\n\t}else{\n"
     "\t\tpc_payzeny(sd, (int32)z, LOG_TYPE_VENDING, vsd->status.char_id);\n\t\tachievement_update_objective(sd, AG_SPEND_ZENY, 1, (int32)z);\n\t\tz = vending_calc_tax(sd, z);\n\t\tpc_getzeny(vsd, (int32)z, LOG_TYPE_VENDING, sd->status.char_id);\n\t}\n")
edit("map/vending.cpp", "\t\tpc_cart_delitem(vsd, idx, amount, 0, LOG_TYPE_VENDING);\n\t\tz = vending_calc_tax(sd, z);",
     "\t\tpc_cart_delitem(vsd, idx, amount, 0, LOG_TYPE_VENDING);\n\t\tif( vsd->vend_currency == 0 ) // HuffLabs: no tax on item payments\n\t\t\tz = vending_calc_tax(sd, z);")

# payment helper + menu, placed before vending_purchasereq's doc comment
edit("map/vending.cpp", "/**\n * Purchase item(s) from a shop\n",
r'''/**
 * HuffLabs vending currency (idea from Lilith/Easycore's Extended Vending, rewritten).
 */
bool vending_currency_enabled(){
	return battle_config.vending_currency_item > 0 && battle_config.vending_zeny_item > 0
		&& item_db.exists( battle_config.vending_currency_item ) && item_db.exists( battle_config.vending_zeny_item );
}

/// Shows the currency choice (Zeny or the currency item) in the item-pick window.
void vending_currency_menu( map_session_data& sd, uint16 skill_lv ){
	clif_vending_currency_list( sd, battle_config.vending_zeny_item, battle_config.vending_currency_item );
	sd.menuskill_id = MC_VENDING;
	sd.menuskill_val = skill_lv;
}

/// The player picked a currency: set up the shop and open the vending window.
void vending_currency_select( map_session_data& sd, t_itemid nameid ){
	if( nameid == static_cast<t_itemid>( battle_config.vending_currency_item ) )
		sd.vend_currency = nameid;
	else if( nameid == static_cast<t_itemid>( battle_config.vending_zeny_item ) )
		sd.vend_currency = 0;
	else{
		clif_skill_fail( sd, MC_VENDING );
		return;
	}

	if( !pc_can_give_items( &sd ) || sd.state.vending || sd.state.buyingstore || sd.state.trading ){
		sd.vend_currency = 0;
		clif_skill_fail( sd, MC_VENDING );
		return;
	}

	int32 i = 0;
	sd.state.prevend = 1;
	sd.state.workinprogress = WIP_DISABLE_ALL;
	ARR_FIND( 0, MAX_CART, i, sd.cart.u.items_cart[i].nameid && sd.cart.u.items_cart[i].id == 0 );
	if( i < MAX_CART ){
		// Save the cart before opening the vending UI
		sd.state.pending_vending_ui = true;
		intif_storage_save( &sd, &sd.cart );
	}else{
		sd.state.pending_vending_ui = false;
		clif_openvendingreq( sd, 2 + sd.vend_skill_lv );
	}
}

/// Moves <price> of the shop's currency item from buyer to vendor. Everything is checked
/// before anything moves; bound and equipped items never count as payment.
static bool vending_pay_with_item( map_session_data& sd, map_session_data& vsd, int32 price ){
	t_itemid cur = vsd.vend_currency;
	std::shared_ptr<item_data> id = item_db.find( cur );

	if( id == nullptr || price < 0 )
		return false;
	if( price == 0 )
		return true;

	int32 have = 0;
	for( int32 i = 0; i < MAX_INVENTORY; i++ ){
		const item& it = sd.inventory.u.items_inventory[i];
		if( it.nameid == cur && it.bound == BOUND_NONE && it.equip == 0 )
			have += it.amount;
	}

	std::string msg;
	if( have < price )
		msg = "You don't have enough " + id->ename + " (need " + std::to_string( price ) + ", have " + std::to_string( have ) + ").";
	else{
		char chk = pc_checkadditem( &vsd, cur, price );
		if( chk == CHKADDITEM_OVERAMOUNT || ( chk == CHKADDITEM_NEW && pc_inventoryblank( &vsd ) == 0 )
			|| vsd.weight + static_cast<int64>( id->weight ) * price > vsd.max_weight )
			msg = "The seller can't carry that much " + id->ename + ".";
	}
	if( !msg.empty() ){
		clif_messagecolor( &sd, color_table[COLOR_RED], msg.c_str(), false, SELF );
		return false;
	}

	item pay = {};
	pay.nameid = cur;
	pay.identify = 1;
	if( pc_additem( &vsd, &pay, price, LOG_TYPE_VENDING ) != ADDITEM_SUCCESS ){
		clif_messagecolor( &sd, color_table[COLOR_RED], ( "The seller can't carry that much " + id->ename + "." ).c_str(), false, SELF );
		return false;
	}

	int32 left = price;
	for( int32 i = 0; i < MAX_INVENTORY && left > 0; i++ ){
		const item& it = sd.inventory.u.items_inventory[i];
		if( it.nameid != cur || it.bound != BOUND_NONE || it.equip != 0 )
			continue;
		int32 take = std::min<int32>( it.amount, left );
		pc_delitem( &sd, i, take, 0, 0, LOG_TYPE_VENDING );
		left -= take;
	}
	return true;
}

/**
 * Purchase item(s) from a shop
''')

# keep the currency for shops that stay open offline (autotrade)
edit("map/vending.cpp", "`body_direction`, `head_direction`, `sit`) \"\n\t\t\"VALUES( %d, %d, %d, '%c', '%s', %d, %d, '%s', %d, '%d', '%d', '%d' );\",",
     "`body_direction`, `head_direction`, `sit`, `hlro_currency`) \"\n\t\t\"VALUES( %d, %d, %d, '%c', '%s', %d, %d, '%s', %d, '%d', '%d', '%d', %u );\",")
edit("map/vending.cpp", "at ? at->sit : pc_issit(&sd) ) != SQL_SUCCESS ) {",
     "at ? at->sit : pc_issit(&sd), sd.vend_currency ) != SQL_SUCCESS ) {")
edit("map/vending.cpp", "\"SELECT `id`, `account_id`, `char_id`, `sex`, `title`, `body_direction`, `head_direction`, `sit` \"",
     "\"SELECT `id`, `account_id`, `char_id`, `sex`, `title`, `body_direction`, `head_direction`, `sit`, `hlro_currency` \"")
edit("map/vending.cpp", "\t\t\t\tSql_GetData(mmysql_handle, 7, &data, nullptr); at->sit = atoi(data);\n",
     "\t\t\t\tSql_GetData(mmysql_handle, 7, &data, nullptr); at->sit = atoi(data);\n\t\t\t\tSql_GetData(mmysql_handle, 8, &data, nullptr); at->currency = strtoul(data, nullptr, 10); // HuffLabs\n")
edit("map/vending.cpp", "\t\tsd.state.prevend = 1; // Set him into a hacked prevend state\n",
     "\t\tsd.state.prevend = 1; // Set him into a hacked prevend state\n\t\tsd.vend_currency = at->currency; // HuffLabs\n")

# clif: the currency pick window (packet helpers are private to clif.cpp)
edit("map/clif.hpp", "void clif_magicdecoy_list( map_session_data& sd, uint16 skill_lv, int16 x, int16 y );\n",
     "void clif_magicdecoy_list( map_session_data& sd, uint16 skill_lv, int16 x, int16 y );\nvoid clif_vending_currency_list( map_session_data& sd, t_itemid zeny_item, t_itemid currency_item ); // HuffLabs\n")
edit("map/clif.cpp", "/*==========================================\n * Guillotine Cross Poisons List\n",
"""/// HuffLabs: vending currency choice, shown in the item-pick window (ZC_MAKINGARROW_LIST).
void clif_vending_currency_list( map_session_data& sd, t_itemid zeny_item, t_itemid currency_item ){
	PACKET_ZC_MAKINGARROW_LIST* p = reinterpret_cast<PACKET_ZC_MAKINGARROW_LIST*>( packet_buffer );

	p->packetType = HEADER_ZC_MAKINGARROW_LIST;
	p->packetLength = sizeof( *p );
	p->items[0].itemId = client_nameid( zeny_item );
	p->items[1].itemId = client_nameid( currency_item );
	p->packetLength += static_cast<decltype(p->packetLength)>( 2 * sizeof( p->items[0] ) );
	clif_send( p, p->packetLength, &sd, SELF );
}

/*==========================================
 * Guillotine Cross Poisons List
""")
edit("map/vending.cpp", '#include "clif.hpp"\n', '#include "clif.hpp"\n#include "intif.hpp" // HuffLabs: intif_storage_save\n')
print("ok")
