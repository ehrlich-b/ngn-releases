//go:build amd64 && amd64.v3

#include "textflag.h"

// affineRowValue1024AVX2 relies on the fc0 input invariant that every unsigned
// byte is at most 127. Therefore each signed pair sum produced by VPMADDUBSW is
// in [-32512, 32258] and cannot saturate.
TEXT ·affineRowValue1024AVX2(SB), NOSPLIT, $0-28
	MOVQ input+0(FP), AX
	MOVQ weights+8(FP), BX
	MOVL $0x00010001, CX
	VMOVD CX, X3
	VPBROADCASTD X3, Y3
	VPXOR Y4, Y4, Y4
	MOVL $32, DX
affine_1024_loop:
	VMOVDQU (AX), Y0
	VMOVDQU (BX), Y1
	VPMADDUBSW Y1, Y0, Y2
	VPMADDWD Y3, Y2, Y2
	VPADDD Y2, Y4, Y4
	ADDQ $32, AX
	ADDQ $32, BX
	DECL DX
	JNZ affine_1024_loop

	VEXTRACTI128 $1, Y4, X5
	VPADDD X5, X4, X4
	VPHADDD X4, X4, X4
	VPHADDD X4, X4, X4
	VMOVD X4, AX
	ADDL bias+16(FP), AX
	MOVL AX, ret+24(FP)
	VZEROUPPER
	RET

// affineLayer32x32AVX2 evaluates all 32 rows while retaining the clipped input
// in a vector register. The same [0, 127] pair-sum bound as fc0 applies.
TEXT ·affineLayer32x32AVX2(SB), NOSPLIT, $0-32
	MOVQ output+0(FP), AX
	MOVQ input+8(FP), BX
	MOVQ weights+16(FP), CX
	MOVQ biases+24(FP), DX
	VMOVDQU (BX), Y0
	MOVL $0x00010001, R9
	VMOVD R9, X3
	VPBROADCASTD X3, Y3
	MOVL $32, R8
affine_32x32_loop:
	VMOVDQU (CX), Y1
	VPMADDUBSW Y1, Y0, Y2
	VPMADDWD Y3, Y2, Y2
	VEXTRACTI128 $1, Y2, X4
	VPADDD X4, X2, X2
	VPHADDD X2, X2, X2
	VPHADDD X2, X2, X2
	VMOVD X2, R9
	ADDL (DX), R9
	MOVL R9, (AX)
	ADDQ $32, CX
	ADDQ $4, DX
	ADDQ $4, AX
	DECL R8
	JNZ affine_32x32_loop
	VZEROUPPER
	RET

TEXT ·affineRowValue32AVX2(SB), NOSPLIT, $0-28
	MOVQ input+0(FP), AX
	MOVQ weights+8(FP), BX
	VMOVDQU (AX), Y0
	VMOVDQU (BX), Y1
	VPMADDUBSW Y1, Y0, Y2
	MOVL $0x00010001, CX
	VMOVD CX, X3
	VPBROADCASTD X3, Y3
	VPMADDWD Y3, Y2, Y2
	VEXTRACTI128 $1, Y2, X4
	VPADDD X4, X2, X2
	VPHADDD X2, X2, X2
	VPHADDD X2, X2, X2
	VMOVD X2, AX
	ADDL bias+16(FP), AX
	MOVL AX, ret+24(FP)
	VZEROUPPER
	RET
